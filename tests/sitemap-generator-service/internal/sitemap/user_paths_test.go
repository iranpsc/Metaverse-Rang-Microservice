package sitemap_test

import (
	"testing"

	"metarang/sitemap-generator-service/internal/sitemap"
)

func TestUserPathURLs(t *testing.T) {
	cases := []struct {
		name string
		got  []string
		want []string
	}{
		{
			name: "wallet",
			got:  sitemap.UserWalletURLs("U1"),
			want: []string{
				"https://metarang.com/fa/citizens/U1/wallet",
				"https://metarang.com/en/citizens/U1/wallet",
			},
		},
		{
			name: "lands",
			got:  sitemap.UserLandsURLs("U1"),
			want: []string{
				"https://metarang.com/fa/citizens/U1/summary",
				"https://metarang.com/en/citizens/U1/summary",
			},
		},
		{
			name: "buildings",
			got:  sitemap.UserBuildingsURLs("U1"),
			want: []string{
				"https://metarang.com/fa/citizens/U1/buildings",
				"https://metarang.com/en/citizens/U1/buildings",
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assertStrings(t, tc.got, tc.want)
		})
	}
}

func TestChunkedFileName(t *testing.T) {
	cases := []struct {
		base  string
		index int
		want  string
	}{
		{"user-wallet-sitemaps.xml", 0, "user-wallet-sitemaps.xml"},
		{"user-wallet-sitemaps.xml", 1, "user-wallet-sitemaps-2.xml"},
		{"user-lands-sitemap.xml", 2, "user-lands-sitemap-3.xml"},
	}
	for _, tc := range cases {
		if got := sitemap.ChunkedFileName(tc.base, tc.index); got != tc.want {
			t.Fatalf("base=%q index=%d got %q want %q", tc.base, tc.index, got, tc.want)
		}
	}
}
