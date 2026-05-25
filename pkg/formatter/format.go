package formatter

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/u3mur4/mail-notify/pkg/mailwatch"
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
func Pango(event mailwatch.UpdateEvent) string {
	buffer := bytes.Buffer{}
	fmt.Fprint(&buffer, "<span>")

	switch event.Status {
	case mailwatch.StatusDisconnected:
		fmt.Fprint(&buffer, "</span>")
		return `<span foreground='gray'></span>`
	case mailwatch.StatusNeedsAuth:
		fmt.Fprint(&buffer, "<span size='large' rise='2000' foreground='red'>⟳</span>")
	case mailwatch.StatusConnected:
		if event.Unseen > 0 {
			fmt.Fprintf(&buffer, "<span size='large' rise='2000' foreground='red'>%s</span>", uInt32ToCircledNumberStr(event.Unseen))
		}
	}

	fmt.Fprint(&buffer, "</span>")
	return buffer.String()
}

func Waybar(event mailwatch.UpdateEvent) string {
	return Pango(event)
}

// TODO: handle unauthenticated state with clickable auth URL
func Polybar(event mailwatch.UpdateEvent, leftClickCmd string) string {
	buffer := bytes.Buffer{}

	// left click
	fmt.Fprint(&buffer, "%{A1:")
	fmt.Fprint(&buffer, strings.Replace(leftClickCmd, ":", "\\:", -1))
	fmt.Fprint(&buffer, ":}")

	fmt.Fprint(&buffer, "")
	if event.Status == mailwatch.StatusConnected && event.Unseen > 0 {
		fmt.Fprint(&buffer, "%{F#f00}%{O-3}")
		fmt.Fprintf(&buffer, "%s", uInt32ToCircledNumberStr(event.Unseen))
		fmt.Fprint(&buffer, "%{F-}%{O}")
	}
	// end left click
	fmt.Fprint(&buffer, "%{A}")
	return buffer.String()
}
