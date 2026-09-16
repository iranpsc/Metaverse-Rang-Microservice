package sadad

// SadadError handles Sadad error codes with Persian messages.
type SadadError struct {
	Code string
}

// NewSadadError creates a new Sadad error.
func NewSadadError(code string) *SadadError {
	return &SadadError{Code: code}
}

// Message returns the Persian error message for the error code.
// Prefer gateway Description when available; this table is the official ResCode fallback.
func (e *SadadError) Message() string {
	switch e.Code {
	case "0":
		return "تراکنش با موفقیت انجام شد"
	case "-1":
		return "تراکنش ناموفق می باشد"
	case "3":
		return "پذیرنده کارت فعال نیست، لطفا با بخش امور پذیرندگان تماس حاصل فرمائید"
	case "23":
		return "پذیرنده کارت نامعتبر است، لطفا با بخش امور پذیرندگان تماس حاصل فرمائید"
	case "58":
		return "انجام تراکنش مربوطه توسط پایانه انجام دهنده مجاز نمی باشد"
	case "61":
		return "مبلغ تراکنش از حد مجاز بالاتر است"
	case "1000":
		return "ترتیب پارامترهای ارسالی اشتباه می باشد"
	case "1001":
		return "پارامترهای پرداخت اشتباه می باشد"
	case "1002":
		return "خطا در سیستم، تراکنش ناموفق"
	case "1003":
		return "IP پذیرنده اشتباه است"
	case "1004":
		return "شماره پذیرنده اشتباه است"
	case "1005":
		return "خطای دسترسی، لطفا بعدا تلاش فرمائید"
	case "1006":
		return "خطا در سیستم"
	case "1011":
		return "درخواست تکراری، شماره سفارش تکراری می باشد"
	case "1012":
		return "اطلاعات پذیرنده صحیح نیست، یکی از موارد تاریخ، زمان یا کلید تراکنش اشتباه است"
	case "1015":
		return "پاسخ خطای نامشخص از سمت مرکز"
	case "1017":
		return "مبلغ درخواستی از حد مجاز تعریف شده برای این پذیرنده بیشتر است"
	case "1018":
		return "اشکال در تاریخ و زمان سیستم، لطفا تاریخ و زمان سرور را با بانک هماهنگ نمایید"
	case "1019":
		return "امکان پرداخت از طریق سیستم شتاب برای این پذیرنده امکان پذیر نیست"
	case "1020":
		return "پذیرنده غیرفعال شده است"
	case "1023":
		return "آدرس بازگشت پذیرنده نامعتبر است"
	case "1024":
		return "مهر زمانی پذیرنده نامعتبر است"
	case "1025":
		return "امضا تراکنش نامعتبر است"
	case "1026":
		return "شماره سفارش تراکنش نامعتبر است"
	case "1027":
		return "شماره پذیرنده نامعتبر است"
	case "1028":
		return "شماره ترمینال پذیرنده نامعتبر است"
	case "1029":
		return "آدرس IP پرداخت در محدوده آدرس های معتبر اعلام شده توسط پذیرنده نیست"
	case "1030":
		return "آدرس Domain پرداخت در محدوده آدرس های معتبر اعلام شده توسط پذیرنده نیست"
	case "1031":
		return "مهلت زمانی پرداخت به پایان رسیده است"
	case "1032":
		return "پرداخت با این کارت برای پذیرنده مورد نظر امکان پذیر نیست"
	case "1033":
		return "به علت مشکل در سایت پذیرنده، پرداخت برای این پذیرنده غیرفعال شده است"
	case "1036":
		return "اطلاعات اضافی ارسال نشده یا دارای اشکال است"
	case "1037":
		return "شماره پذیرنده یا شماره ترمینال پذیرنده صحیح نمی باشد"
	case "1040":
		return "شناسه وارد شده معتبر نمی باشد"
	case "1053":
		return "درخواست معتبر از سمت پذیرنده صورت نگرفته است"
	case "1055":
		return "مقدار غیرمجاز در ورود اطلاعات"
	case "1056":
		return "سیستم موقتا قطع می باشد"
	case "1058":
		return "سرویس پرداخت اینترنتی خارج از سرویس می باشد"
	case "1061":
		return "اشکال در تولید کد یکتا"
	case "1064":
		return "لطفا مجددا سعی بفرمایید"
	case "1065":
		return "ارتباط ناموفق، لطفا چند لحظه دیگر مجددا سعی کنید"
	case "1066":
		return "سیستم سرویس دهی پرداخت موقتا غیر فعال شده است"
	case "1068":
		return "به علت بروزرسانی، سیستم موقتا قطع می باشد"
	case "1072":
		return "خطا در پردازش پارامترهای اختیاری پذیرنده"
	case "1101":
		return "مبلغ تراکنش نامعتبر است"
	case "1103":
		return "توکن ارسالی نامعتبر است"
	case "1104":
		return "اطلاعات تسهیم صحیح نیست"
	case "1105":
		return "تراکنش بازگشت داده شده است"
	// Legacy short codes kept for older clients/tests.
	case "101":
		return "پذیرنده نامعتبر است"
	case "102":
		return "ترمینال نامعتبر است"
	case "103":
		return "مبلغ نامعتبر است"
	case "104":
		return "شناسه سفارش تکراری است"
	case "105":
		return "امضای دیجیتال نامعتبر است"
	case "106":
		return "توکن نامعتبر است"
	case "107":
		return "تراکنش قبلا تایید شده است"
	default:
		return "خطای ناشناخته در درگاه پرداخت"
	}
}

// GetCode returns the error code.
func (e *SadadError) GetCode() string {
	return e.Code
}

// IsSuccess checks if the code indicates success.
func (e *SadadError) IsSuccess() bool {
	return e.Code == "0"
}
