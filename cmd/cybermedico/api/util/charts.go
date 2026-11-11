package util

import "context"

func GetChartTheme(ctx context.Context) string {
	uiTheme := GetTheme(ctx)

	switch uiTheme {
	case "dark":
		return "dark"
	case "light":
		return "white"
	case "auto":
		return "white"
	default:
		return "white"
	}
}
