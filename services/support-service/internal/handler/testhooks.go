package handler

// Test hooks for external test modules (tests/support-service).

var (
	ExportWriteJSON                            = writeJSON
	ExportWriteHandlerError                    = writeHandlerError
	ExportExtractIDFromPath                    = extractIDFromPath
	ExportSplitJalaliDateTime                  = splitJalaliDateTime
	ExportDecodeJSONBody                       = decodeJSONBody
	ExportSpoofedMethodFromValues              = spoofedMethodFromValues
	ExportUploadTicketAttachment               = uploadTicketAttachment
	ExportUploadBytesToStorage                 = uploadBytesToStorage
	ExportParseTicketFormFields                = parseTicketFormFields
	ExportParseNoteFormFields                  = parseNoteFormFields
	ExportParseReportFormFields                = parseReportFormFields
	ExportUploadReportAttachments              = uploadReportAttachments
	ExportResolveNoteAttachmentURL             = resolveNoteAttachmentURL
	ExportUploadBytesToStorageWithRelativePath = uploadBytesToStorageWithRelativePath
	ExportRelativeDBPath                       = relativeDBPath
	ExportPrependPublicURL                     = prependPublicURL
)

const (
	ExportMaxTicketAttachmentSize = maxTicketAttachmentSize
	ExportMaxReportAttachmentSize = maxReportAttachmentSize
)

// FileStorageUploader is exported for tests that stub storage-service uploads.
type FileStorageUploader = fileStorageUploader
