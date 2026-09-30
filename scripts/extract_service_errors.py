#!/usr/bin/env python3
"""Extract validation, 403, and response messages from services/."""
import json
import os
import re
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
SERVICES = ROOT / "services"
OUT = ROOT / "docs" / "services-error-catalog.md"


def snake_from_err(name: str) -> str:
    if name.startswith("Err"):
        name = name[3:]
    s = re.sub(r"([a-z])([A-Z])", r"\1_\2", name)
    s = re.sub(r"([A-Z]+)([A-Z][a-z])", r"\1_\2", s)
    return s.upper()


def slug(text: str, max_len: int = 55) -> str:
    s = re.sub(r"[^a-z0-9]+", "_", text.lower()).strip("_")
    return s[:max_len] if s else "unknown"


HTTP_STATUS = {
    "Forbidden": "403",
    "Unauthorized": "401",
    "BadRequest": "400",
    "NotFound": "404",
    "MethodNotAllowed": "405",
    "Conflict": "409",
    "PreconditionFailed": "412",
    "UnprocessableEntity": "422",
    "InternalServerError": "500",
    "ServiceUnavailable": "503",
}

GRPC_TO_HTTP = {
    "PermissionDenied": "403",
    "Unauthenticated": "401",
    "InvalidArgument": "422",
    "NotFound": "404",
    "FailedPrecondition": "412",
    "AlreadyExists": "409",
    "Conflict": "409",
    "Unavailable": "503",
}


def is_english(text: str) -> bool:
    return not any("\u0600" <= c <= "\u06FF" for c in text)


entries: list[dict] = []


def infer_http_code(message: str, category: str, string_key: str) -> int:
    if category == "403":
        return 403
    if category == "validation":
        return 422
    if category.isdigit():
        return int(category)

    m = re.match(r"HTTP_(\d+)_", string_key)
    if m:
        return int(m.group(1))

    if string_key.startswith("GRPC_"):
        grpc = string_key.removeprefix("GRPC_").split("_", 1)[0]
        if grpc in GRPC_TO_HTTP:
            return int(GRPC_TO_HTTP[grpc])

    if string_key == "DYNASTY_INVALID_RELATIONSHIP":
        return 400

    lower = message.lower()
    if "not found" in lower:
        return 404
    if any(
        x in lower
        for x in (
            "unauthorized",
            "permission",
            "does not belong",
            "forbidden",
            "not allowed",
            "cannot follow yourself",
            "already following",
            "profile limitation",
            "does not own",
            "csrf token",
            "not verified.",
        )
    ):
        return 403
    if any(
        x in lower
        for x in (
            "already exists",
            "already linked",
            "already connected",
            "already has",
            "duplicate",
            "already in use",
            "already answered",
            "already following",
            "already unlocked",
            "already reported",
        )
    ):
        return 409
    if "rate limit" in lower or "limit exceeded" in lower:
        return 422
    if "method not allowed" in lower:
        return 405
    if any(
        x in lower
        for x in (
            "invalid",
            "required",
            "must be",
            "must not",
            "exceeds",
            "mutually exclusive",
            "is required",
        )
    ):
        return 422
    if any(
        x in lower
        for x in ("unavailable", "not configured", "not initialized", "temporarily unavailable")
    ):
        return 503
    if any(x in lower for x in ("payment outcome unknown", "needs reconciliation", "failed precondition")):
        return 412
    if any(x in lower for x in ("failed", "internal server")):
        return 500
    return 400


def add(string_key: str, message: str, category: str, http_code: int | None = None) -> None:
    message = message.strip()
    if not message or len(message) > 400:
        return
    if not is_english(message):
        return
    if message in ("%s", "%v", "%d"):
        return
    if re.fullmatch(r"%[svd]", message):
        return
    code = http_code if http_code is not None else infer_http_code(message, category, string_key)
    entries.append({"code": code, "message": message, "_category": category})


def scan_go_files():
    pat_err = re.compile(
        r"(?:var\s+)?(Err[A-Za-z0-9_]+)\s*=\s*errors\.New\(\"((?:\\.|[^\"])*)\"\)"
    )
    pat_inline = re.compile(r'return[^;]*errors\.New\(\"((?:\\.|[^\"])*)\"\)')
    pat_we = re.compile(
        r'writeError\(w,\s*(?:http\.)?Status(\w+),\s*"((?:\\.|[^\"])*)"'
    )
    pat_we_num = re.compile(r'writeError\(w,\s*(\d+),\s*"((?:\\.|[^\"])*)"')
    pat_grpc = re.compile(
        r'status\.Error(?:f)?\(codes\.(\w+),\s*"((?:\\.|[^\"])*)"'
    )
    pat_fmt = re.compile(
        r'fmt\.Errorf\(\"((?:\\.|[^\"%])*)\"'
    )

    for path in SERVICES.rglob("*.go"):
        if "_test.go" in path.name or "vendor" in path.parts:
            continue
        rel = path.relative_to(SERVICES)
        try:
            text = path.read_text(encoding="utf-8")
        except OSError:
            continue

        for m in pat_err.finditer(text):
            msg = bytes(m.group(2), "utf-8").decode("unicode_escape")
            add(snake_from_err(m.group(1)), msg, str(rel))

        for m in pat_inline.finditer(text):
            msg = bytes(m.group(1), "utf-8").decode("unicode_escape")
            add(f"INLINE_{slug(msg)}", msg, str(rel))

        for m in pat_we.finditer(text):
            status = m.group(1)
            msg = m.group(2)
            http = HTTP_STATUS.get(status, status)
            http_code = int(http) if str(http).isdigit() else None
            add(f"HTTP_{http}_{slug(msg)}", msg, str(http), http_code=http_code)

        for m in pat_we_num.finditer(text):
            http, msg = m.group(1), m.group(2)
            add(f"HTTP_{http}_{slug(msg)}", msg, http, http_code=int(http))

        for m in pat_grpc.finditer(text):
            grpc, msg = m.group(1), m.group(2)
            http_code = int(GRPC_TO_HTTP[grpc]) if grpc in GRPC_TO_HTTP else None
            add(f"GRPC_{grpc}_{slug(msg)}", msg, grpc, http_code=http_code)

        for m in pat_fmt.finditer(text):
            msg = m.group(1)
            if "%" in msg:
                continue
            add(f"DOMAIN_{slug(msg)}", msg, str(rel))


def add_validation_templates():
    templates = [
        ("VALIDATION_REQUIRED", "The %s field is required"),
        ("VALIDATION_EMAIL", "The %s field must be a valid email address"),
        ("VALIDATION_MIN", "The %s field must be at least %s characters"),
        ("VALIDATION_MAX", "The %s field must not exceed %s characters"),
        ("VALIDATION_LEN", "The %s field must be exactly %s characters"),
        ("VALIDATION_ONE_OF", "The %s field must be one of: %s"),
        ("VALIDATION_UNIQUE", "The %s field must be unique"),
        ("VALIDATION_PERSIAN", "The %s field must contain only Persian characters"),
        (
            "VALIDATION_PERSIAN_ALPHA",
            "The %s field must contain only Persian alphabetic characters",
        ),
        ("VALIDATION_PERSIAN_NUM", "The %s field must contain only Persian numbers"),
        (
            "VALIDATION_PERSIAN_ALPHA_NUM",
            "The %s field must contain only Persian alphanumeric characters",
        ),
        (
            "VALIDATION_IRANIAN_MOBILE",
            "The %s field must be a valid Iranian mobile number",
        ),
        (
            "VALIDATION_IRANIAN_POSTAL_CODE",
            "The %s field must be a valid Iranian postal code",
        ),
        (
            "VALIDATION_IRANIAN_NATIONAL_CODE",
            "The %s field must be a valid Iranian national code",
        ),
        (
            "VALIDATION_IRANIAN_SHEBA",
            "The %s field must be a valid Iranian Sheba (IBAN) number",
        ),
        (
            "VALIDATION_IRANIAN_BANK_CARD",
            "The %s field must be a valid Iranian bank card number",
        ),
        ("VALIDATION_INVALID", "The %s field is invalid"),
    ]
    for code, msg in templates:
        add(code, msg, "validation")


def lang_value_looks_like_error(text: str) -> bool:
    lower = text.lower()
    markers = (
        "failed",
        "required",
        "not found",
        "unauthorized",
        "invalid",
        "error",
        "exceed",
        "must be",
        "must not",
        "not available",
        "not belong",
        "limit exceeded",
        "not verified",
        "not initialized",
    )
    return any(m in lower for m in markers)


def add_lang_en():
    for path in SERVICES.rglob("internal/lang/en.json"):
        try:
            data = json.loads(path.read_text(encoding="utf-8"))
        except (OSError, json.JSONDecodeError):
            continue
        svc = path.parts[path.parts.index("services") + 1]
        for key, val in data.items():
            if not is_english(str(val)):
                continue
            if "%v" in val or "%s" in val:
                continue
            if not lang_value_looks_like_error(str(val)):
                continue
            add(f"LANG_{svc.upper()}_{slug(key)}", val, "lang")


def add_dynasty_403_english():
    """Persian dynasty validation messages mapped to English (HTTP 403)."""
    dynasty_403 = [
        ("DYNASTY_DM_PERMISSION_DENIED", "You are not allowed to manage the dynasty."),
        ("DYNASTY_CANNOT_REQUEST_SELF", "You cannot send a join request to yourself."),
        ("DYNASTY_NOT_ESTABLISHED", "You have not established a dynasty."),
        ("DYNASTY_PENDING_REQUEST_EXISTS", "You have already sent a request to this user."),
        ("DYNASTY_REQUEST_REJECTED", "Your request was previously rejected by this user."),
        ("DYNASTY_USER_ALREADY_IN_FAMILY", "This user is already in your dynasty."),
        ("DYNASTY_FATHER_LIMIT", "You can only have one father."),
        ("DYNASTY_MOTHER_LIMIT", "You can only have one mother."),
        ("DYNASTY_HUSBAND_LIMIT", "You can only have one husband."),
        ("DYNASTY_WIFE_LIMIT", "You can only have four wives."),
        ("DYNASTY_OFFSPRING_LIMIT", "You already have more than 4 members in your dynasty."),
        ("DYNASTY_INVALID_RELATIONSHIP", "Invalid relationship type."),
    ]
    for code, msg in dynasty_403:
        http = 400 if code == "DYNASTY_INVALID_RELATIONSHIP" else 403
        add(code, msg, str(http), http_code=http)


def add_custom_validation_messages():
    custom = [
        ("VALIDATION_PRICE_MUTUAL_EXCLUSIVE", "price_psc/price_irr and minimum_price_percentage are mutually exclusive"),
        ("VALIDATION_PRICE_REQUIRED", "either price_psc/price_irr or minimum_price_percentage"),
        ("VALIDATION_PERIOD_REQUIRED", "The period field is required."),
        ("VALIDATION_PERIOD_INVALID", "The selected period is invalid."),
        ("VALIDATION_ASSETS_INVALID", "The selected assets is invalid."),
        ("VALIDATION_QUESTION_ID_REQUIRED", "question_id is required"),
        ("VALIDATION_ANSWER_ID_REQUIRED", "answer_id is required"),
        ("VALIDATION_SEARCH_TERM_REQUIRED", "searchTerm is required"),
        ("VALIDATION_RELATIONSHIP_OFFSPRING", "relationship must be 'offspring'"),
        ("VALIDATION_PERMISSION_CODE_INVALID", "invalid permission code"),
        ("VALIDATION_MOBILE_RESET_LIMIT", "You have reached the maximum number of mobile number changes"),
    ]
    for code, msg in custom:
        add(code, msg, "validation")


def write_markdown():
    OUT.parent.mkdir(parents=True, exist_ok=True)
    entries.sort(key=lambda e: e["code"])
    lines = [
        "# Services error catalog",
        "",
        "Catalog of validation, HTTP 403 (`PermissionDenied`), and other English API/gRPC response messages under `services/`.",
        "",
        "`code` is the HTTP status returned to clients (inferred from handlers / gRPC mapping when not explicit).",
        "",
        "Regenerate: `python scripts/extract_service_errors.py`",
        "",
        "Dynasty family rules are stored in Persian in code; English equivalents are listed with status **403** (or **400** for invalid relationship).",
        "",
        "## Entries",
        "",
        "```json",
        "[",
    ]
    for i, e in enumerate(entries):
        obj = json.dumps({"code": e["code"], "message": e["message"]}, ensure_ascii=False)
        suffix = "," if i < len(entries) - 1 else ""
        lines.append(f"  {obj}{suffix}")
    lines.append("]")
    lines.append("```")
    lines.append("")
    OUT.write_text("\n".join(lines), encoding="utf-8")
    print(f"Wrote {len(entries)} entries to {OUT}")


def main():
    global entries
    scan_go_files()
    add_validation_templates()
    add_lang_en()
    add_dynasty_403_english()
    add_custom_validation_messages()
    # dedupe by message (keep first code)
    by_msg: dict[str, dict] = {}
    for e in entries:
        if e["message"] not in by_msg:
            by_msg[e["message"]] = e
    entries = sorted(by_msg.values(), key=lambda x: (x["code"], x["message"]))
    write_markdown()


if __name__ == "__main__":
    main()
