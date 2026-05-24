package formatter

import (
	"bytes"
	"fmt"
	"strings"
)

// uInt32ToCircledNumberStr converts int to unicode circled number
func uInt32ToCircledNumberStr(number uint32) string {
	// cache := map[uint32]string{
	// 	0:  "🄌",
	// 	1:  "➊",
	// 	2:  "➋",
	// 	3:  "➌",
	// 	4:  "➍",
	// 	5:  "➎",
	// 	6:  "➏",
	// 	7:  "➐",
	// 	8:  "➑",
	// 	9:  "➒",
	// 	10: "➓",
	// 	11: "⓫",
	// 	12: "⓬",
	// 	13: "⓭",
	// 	14: "⓮",
	// 	15: "⓯",
	// 	16: "⓰",
	// 	17: "⓱",
	// 	18: "⓲",
	// 	19: "⓳",
	// 	20: "⓴",
	// }

	// if number > 20 {
	// 	return cache[20] + "⁺"
	// }
	// return cache[number]

	cache := map[string]string{
		"0": "🄌",
		"1": "➊",
		"2": "➋",
		"3": "➌",
		"4": "➍",
		"5": "➎",
		"6": "➏",
		"7": "➐",
		"8": "➑",
		"9": "➒",
	}

	result := strings.Builder{}
	for _, ch := range fmt.Sprintf("%d", number) {
		if newch, ok := cache[string(ch)]; ok {
			result.WriteString(newch)
		} else {
			result.WriteString(string(ch))
		}
	}
	return result.String()
}

// formatMailAsPango formats the email number to pango format that is usable by i3blocks
func Pango(unseen uint32, authenticated bool) string {
	buffer := bytes.Buffer{}
	fmt.Fprint(&buffer, "<span>")
	if !authenticated {
		fmt.Fprint(&buffer, "<span size='large' rise='2000' foreground='red'>⟳</span>")
	} else if unseen > 0 {
		fmt.Fprintf(&buffer, "<span size='large' rise='2000' foreground='red'>%s</span>", uInt32ToCircledNumberStr(unseen))
	}
	fmt.Fprint(&buffer, "</span>")
	return buffer.String()
}

func Waybar(unseen uint32, authenticated bool) string {
	return Pango(unseen, authenticated)
}

// TODO: handle unauthenticated state with clickable auth URL
func Polybar(unseen uint32, leftClickCmd string, authenticated bool) string {
	buffer := bytes.Buffer{}

	// left click
	fmt.Fprint(&buffer, "%{A1:")
	fmt.Fprint(&buffer, strings.Replace(leftClickCmd, ":", "\\:", -1))
	fmt.Fprint(&buffer, ":}")

	fmt.Fprint(&buffer, "")
	if unseen > 0 {
		fmt.Fprint(&buffer, "%{F#f00}%{O-3}")
		fmt.Fprintf(&buffer, "%s", uInt32ToCircledNumberStr(unseen))
		fmt.Fprint(&buffer, "%{F-}%{O}")
	}
	// end left click
	fmt.Fprint(&buffer, "%{A}")
	return buffer.String()
}
