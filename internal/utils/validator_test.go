package utils

import "testing"

func TestIsValidMagnetLink(t *testing.T) {
	cases := []struct {
		name   string
		magnet string
		want   bool
	}{
		{
			"valid hex info-hash",
			"magnet:?xt=urn:btih:0123456789abcdef0123456789abcdef01234567",
			true,
		},
		{
			"valid uppercase hex info-hash",
			"magnet:?xt=urn:btih:0123456789ABCDEF0123456789ABCDEF01234567",
			true,
		},
		{
			"valid base32 info-hash",
			"magnet:?xt=urn:btih:ABCDEFGHIJKLMNOPQRSTUVWXYZ234567",
			true,
		},
		{"missing magnet prefix", "https://example.com", false},
		{"missing xt param", "magnet:?dn=something", false},
		{"xt not a btih urn", "magnet:?xt=urn:sha1:0123456789abcdef0123456789abcdef01234567", false},
		{"hash too short", "magnet:?xt=urn:btih:0123456789", false},
		{"hash wrong charset", "magnet:?xt=urn:btih:!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!!", false},
		{"empty string", "", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := IsValidMagnetLink(c.magnet); got != c.want {
				t.Errorf("IsValidMagnetLink(%q) = %v, want %v", c.magnet, got, c.want)
			}
		})
	}
}

func TestIsVideoFile(t *testing.T) {
	cases := map[string]bool{
		"movie.mp4":         true,
		"movie.mkv":         true,
		"movie.avi":         true,
		"movie.mov":         true,
		"MOVIE.MP4":         true,
		"path/to/movie.mkv": true,
		"readme.txt":        false,
		"movie.srt":         false,
		"noextension":       false,
	}
	for path, want := range cases {
		if got := IsVideoFile(path); got != want {
			t.Errorf("IsVideoFile(%q) = %v, want %v", path, got, want)
		}
	}
}

func TestParseLangs(t *testing.T) {
	cases := map[string][]string{
		"en":          {"en"},
		"en,pt-BR":    {"en", "pt-BR"},
		"en,pt-BR,fr": {"en", "pt-BR", "fr"},
	}
	for input, want := range cases {
		got := ParseLangs(input)
		if len(got) != len(want) {
			t.Fatalf("ParseLangs(%q) = %v, want %v", input, got, want)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Errorf("ParseLangs(%q) = %v, want %v", input, got, want)
			}
		}
	}
}
