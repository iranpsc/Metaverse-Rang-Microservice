package service

var karbariLabels = map[string]string{
	"a": "آموزشی",
	"t": "تجاری",
	"m": "مسکونی",
	"e": "اداری",
	"b": "بهداشتی",
	"s": "فضای سبز",
	"f": "فرهنگی",
	"p": "پارکینگ",
	"z": "مذهبی",
	"n": "نمایشگاه",
	"g": "گردشگری",
}

// KarbariLabel returns the Persian label for a karbari code.
func KarbariLabel(karbari string) string {
	if label, ok := karbariLabels[karbari]; ok {
		return label
	}
	return "نامشخص"
}
