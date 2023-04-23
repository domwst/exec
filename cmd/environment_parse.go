package cmd

import (
	"strings"
)

func parseLine(line string) (key, val string) {
	splits := strings.Split(line, "=")
	key = splits[0]
	val = strings.Join(splits[1:], "=")
	return
}

func ParseEnvironment(data []string) map[string]string {
	items := make(map[string]string)
	for _, item := range data {
		key, val := parseLine(item)
		items[key] = val
	}
	return items
}
