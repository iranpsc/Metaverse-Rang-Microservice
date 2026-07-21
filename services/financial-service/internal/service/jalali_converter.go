package service

import (
	"time"

	sharedhelpers "metarang/shared/pkg/helpers"
)

type JalaliConverter interface {
	NowJalali() string
	FormatJalaliDate(t time.Time) string
}

type jalaliConverter struct{}

func NewJalaliConverter() JalaliConverter {
	return &jalaliConverter{}
}

func (c *jalaliConverter) NowJalali() string {
	return sharedhelpers.NowJalali()
}

func (c *jalaliConverter) FormatJalaliDate(t time.Time) string {
	return sharedhelpers.FormatJalaliDate(t)
}
