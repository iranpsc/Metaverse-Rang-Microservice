package handler

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"unicode/utf8"

	pb "metarang/shared/pb/auth"
	"metarang/shared/pkg/helpers"
)

var profileLimitationOptionKeys = []string{
	"follow",
	"send_message",
	"share",
	"send_ticket",
	"view_profile_images",
	"view_features_locations",
}

type profileLimitationNoteInput struct {
	Present bool
	Value   *string // nil when Present means explicit JSON null (clear)
}

type createProfileLimitationInput struct {
	LimitedUserID uint64
	Options       *pb.ProfileLimitationOptions
	Note          profileLimitationNoteInput
}

type updateProfileLimitationInput struct {
	Options *pb.ProfileLimitationOptions
	Note    profileLimitationNoteInput
}

func parseCreateProfileLimitationBody(r *http.Request) (*createProfileLimitationInput, map[string]string) {
	raw, err := readJSONObject(r)
	if err != nil {
		if err == io.EOF {
			return nil, map[string]string{"": "request body is required"}
		}
		return nil, map[string]string{"": "invalid request body"}
	}

	fieldErrors := map[string]string{}
	result := &createProfileLimitationInput{}

	limitedRaw, hasLimited := raw["limited_user_id"]
	if !hasLimited {
		fieldErrors["limited_user_id"] = "The limited user id field is required."
	} else {
		id, ok := decodePositiveUint64(limitedRaw)
		if !ok {
			fieldErrors["limited_user_id"] = "The selected limited user id is invalid."
		} else {
			result.LimitedUserID = id
		}
	}

	options, optionErrors := parseRequiredProfileLimitationOptions(raw)
	for k, v := range optionErrors {
		fieldErrors[k] = v
	}
	result.Options = options

	note, noteErrors := parseOptionalNote(raw)
	for k, v := range noteErrors {
		fieldErrors[k] = v
	}
	result.Note = note

	if len(fieldErrors) > 0 {
		return nil, fieldErrors
	}
	return result, nil
}

func parseUpdateProfileLimitationBody(r *http.Request) (*updateProfileLimitationInput, map[string]string) {
	raw, err := readJSONObject(r)
	if err != nil {
		if err == io.EOF {
			return nil, map[string]string{"": "request body is required"}
		}
		return nil, map[string]string{"": "invalid request body"}
	}

	fieldErrors := map[string]string{}
	result := &updateProfileLimitationInput{}

	options, optionErrors := parseRequiredProfileLimitationOptions(raw)
	for k, v := range optionErrors {
		fieldErrors[k] = v
	}
	result.Options = options

	note, noteErrors := parseOptionalNote(raw)
	for k, v := range noteErrors {
		fieldErrors[k] = v
	}
	result.Note = note

	if len(fieldErrors) > 0 {
		return nil, fieldErrors
	}
	return result, nil
}

func parseRequiredProfileLimitationOptions(raw map[string]json.RawMessage) (*pb.ProfileLimitationOptions, map[string]string) {
	fieldErrors := map[string]string{}

	optionsRaw, hasOptions := raw["options"]
	if !hasOptions || string(optionsRaw) == "null" {
		fieldErrors["options"] = "The options field is required."
		return nil, fieldErrors
	}

	var optionsMap map[string]json.RawMessage
	if err := json.Unmarshal(optionsRaw, &optionsMap); err != nil {
		fieldErrors["options"] = "The options field must be an object."
		return nil, fieldErrors
	}

	allowed := make(map[string]struct{}, len(profileLimitationOptionKeys))
	for _, key := range profileLimitationOptionKeys {
		allowed[key] = struct{}{}
	}

	for key := range optionsMap {
		if _, ok := allowed[key]; !ok {
			fieldErrors[fmt.Sprintf("options.%s", key)] = fmt.Sprintf("The selected options.%s is invalid.", key)
		}
	}

	opts := &pb.ProfileLimitationOptions{}
	for _, key := range profileLimitationOptionKeys {
		valRaw, ok := optionsMap[key]
		if !ok {
			fieldErrors[fmt.Sprintf("options.%s", key)] = fmt.Sprintf("The options.%s field is required.", key)
			continue
		}
		b, ok := decodeStrictBool(valRaw)
		if !ok {
			fieldErrors[fmt.Sprintf("options.%s", key)] = fmt.Sprintf("The options.%s field must be true or false.", key)
			continue
		}
		switch key {
		case "follow":
			opts.Follow = &b
		case "send_message":
			opts.SendMessage = &b
		case "share":
			opts.Share = &b
		case "send_ticket":
			opts.SendTicket = &b
		case "view_profile_images":
			opts.ViewProfileImages = &b
		case "view_features_locations":
			opts.ViewFeaturesLocations = &b
		}
	}

	if len(fieldErrors) > 0 {
		return nil, fieldErrors
	}
	return opts, nil
}

func parseOptionalNote(raw map[string]json.RawMessage) (profileLimitationNoteInput, map[string]string) {
	noteRaw, hasNote := raw["note"]
	if !hasNote {
		return profileLimitationNoteInput{Present: false}, nil
	}
	if string(noteRaw) == "null" {
		return profileLimitationNoteInput{Present: true, Value: nil}, nil
	}

	var note string
	if err := json.Unmarshal(noteRaw, &note); err != nil {
		return profileLimitationNoteInput{}, map[string]string{"note": "The note field must be a string."}
	}
	if utf8.RuneCountInString(note) > 500 {
		return profileLimitationNoteInput{}, map[string]string{"note": "The note may not be greater than 500 characters."}
	}
	return profileLimitationNoteInput{Present: true, Value: &note}, nil
}

func readJSONObject(r *http.Request) (map[string]json.RawMessage, error) {
	defer func() { _ = r.Body.Close() }()
	decoder := json.NewDecoder(r.Body)
	var raw map[string]json.RawMessage
	if err := decoder.Decode(&raw); err != nil {
		return nil, err
	}
	return raw, nil
}

func decodeStrictBool(raw json.RawMessage) (bool, bool) {
	var b bool
	if err := json.Unmarshal(raw, &b); err != nil {
		return false, false
	}
	// Reject numeric 0/1 which json.Unmarshal accepts into bool in some cases?
	// encoding/json does NOT accept numbers into bool; it accepts only true/false.
	return b, true
}

func decodePositiveUint64(raw json.RawMessage) (uint64, bool) {
	var f float64
	if err := json.Unmarshal(raw, &f); err != nil {
		return 0, false
	}
	if f <= 0 || f != float64(uint64(f)) {
		return 0, false
	}
	return uint64(f), true
}

func writeProfileLimitationValidationErrors(w http.ResponseWriter, fieldErrors map[string]string, locale string) {
	// Drop empty-key generic body errors into message-only response
	if msg, ok := fieldErrors[""]; ok && len(fieldErrors) == 1 {
		helpers.WriteValidationErrorResponseFromString(w, msg, locale)
		return
	}
	cleaned := make(map[string]string, len(fieldErrors))
	for k, v := range fieldErrors {
		if k == "" {
			continue
		}
		cleaned[k] = v
	}
	if len(cleaned) == 0 {
		helpers.WriteValidationErrorResponseFromString(w, "The given data was invalid.", locale)
		return
	}
	helpers.WriteValidationErrorResponseFromMap(w, cleaned, locale)
}

func profileLimitationResourceJSON(data *pb.ProfileLimitation, callerUserID uint64) map[string]interface{} {
	resource := map[string]interface{}{
		"id":              data.Id,
		"limiter_user_id": data.LimiterUserId,
		"limited_user_id": data.LimitedUserId,
		"options": map[string]bool{
			"follow":                  data.Options.GetFollow(),
			"send_message":            data.Options.GetSendMessage(),
			"share":                   data.Options.GetShare(),
			"send_ticket":             data.Options.GetSendTicket(),
			"view_profile_images":     data.Options.GetViewProfileImages(),
			"view_features_locations": data.Options.GetViewFeaturesLocations(),
		},
	}

	// Laravel ProfileLimitationResource: include note (even null) only for limiter.
	if callerUserID == data.LimiterUserId {
		if data.Note == nil || *data.Note == "" {
			resource["note"] = nil
		} else {
			resource["note"] = *data.Note
		}
	}

	return resource
}

func notePtrFromInput(note profileLimitationNoteInput) *string {
	if !note.Present {
		return nil
	}
	if note.Value == nil {
		empty := ""
		return &empty
	}
	return note.Value
}

// ParseCreateProfileLimitationBodyForTest exposes parseCreateProfileLimitationBody for gateway tests.
func ParseCreateProfileLimitationBodyForTest(r *http.Request) (*createProfileLimitationInput, map[string]string) {
	return parseCreateProfileLimitationBody(r)
}

func ParseUpdateProfileLimitationBodyForTest(r *http.Request) (*updateProfileLimitationInput, map[string]string) {
	return parseUpdateProfileLimitationBody(r)
}

func ProfileLimitationResourceJSONForTest(data *pb.ProfileLimitation, callerUserID uint64) map[string]interface{} {
	return profileLimitationResourceJSON(data, callerUserID)
}
