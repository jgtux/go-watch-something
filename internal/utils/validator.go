package utils

import (
	"net/url"
	"regexp"
	"strings"
	"path/filepath"
)

func IsValidMagnetLink(magnet string) bool {
	if !strings.HasPrefix(magnet, "magnet:?") {
		return false
	}
	u, err := url.Parse(magnet)
	if err != nil {
		return false
	}
	values, _ := url.ParseQuery(u.RawQuery)
	xt := values.Get("xt")
	if xt == "" || !strings.HasPrefix(xt, "urn:btih:") {
		return false
	}
	hash := xt[len("urn:btih:"):]
	isHex, _ := regexp.MatchString("^[a-fA-F0-9]{40}$", hash)
	isBase32, _ := regexp.MatchString("^[A-Z2-7]{32}$", strings.ToUpper(hash))
	return isHex || isBase32
}

func IsVideoFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return ext == ".mp4" || ext == ".mkv" || ext == ".avi" || ext == ".mov"
}

func ParseLangs(langStr string) []string {
	return strings.Split(langStr, ",")
}
