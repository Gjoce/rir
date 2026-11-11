package widgets

import (
	"context"
	"cybermedico/cmd/cybermedico/api/log"
	"cybermedico/cmd/cybermedico/api/util"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/a-h/templ"
	"github.com/go-echarts/go-echarts/v2/charts"
	"github.com/go-echarts/go-echarts/v2/opts"
)

type TextWidget struct{}

func (t TextWidget) Render(ctx context.Context, config Configuration, data Data) templ.Component {
	if errMsg := data.Error; errMsg != "" {
		return renderWidgetErrorTempl(errMsg)
	}

	if len(data.Rows) == 0 && len(data.Columns) != 0 {
		return renderNoData()
	}

	if len(data.Columns) == 0 {
		return renderSpinner()
	}

	chartHTML := t.generateTextChart(ctx, config, data)

	chartHTML = strings.Replace(chartHTML, `class="container"`, `style="height: 100%;"`, 1)

	return textWidgetTempl(chartHTML)
}

func (t *TextWidget) generateTextChart(ctx context.Context, config Configuration, data Data) string {
	chart := charts.NewLine()

	primaryText := data.Rows[0]["text"]
	secondaryText := data.Rows[0]["subtext"]

	chartDataJson := data.Rows[0]["data"]

	if chartDataJson == "" {
		chartDataJson = "[]"
	}

	// Each point carries its own pre-formatted date (DD/MM/YYYY) so the tooltip
	// can show it instead of the bare X index. The query is expected to emit
	// objects of this shape (see the seeded text widgets in compose.sql).
	chartData := []struct {
		Date  string `json:"date"`
		Value int    `json:"value"`
	}{}

	err := json.Unmarshal([]byte(chartDataJson), &chartData)
	if err != nil {
		log.Logger.ErrorContext(ctx, "couldn't unmarshal chart data", "error", err)
	}

	textFontSize := config.TextFontSize
	subFontSize := config.SubtextFontSize

	theme := util.GetChartTheme(ctx)

	chart.SetGlobalOptions(
		charts.WithInitializationOpts(opts.Initialization{
			Width:    "100%",
			Height:   "100%",
			Theme:    theme,
			Renderer: "svg",
		}),
		charts.WithTitleOpts(opts.Title{
			Title:    primaryText,
			Subtitle: secondaryText,
			Left:     "50%",
			Top:      "middle",
			TitleStyle: &opts.TextStyle{
				FontSize: textFontSize,
			},
			SubtitleStyle: &opts.TextStyle{
				FontSize: subFontSize,
			},
			TextAlign:         "center",
			TextVerticalAlign: "middle",
		}),
		charts.WithTooltipOpts(opts.Tooltip{
			Show:    opts.Bool(true),
			Trigger: "axis",
			// First row: the point's date (dim 2 of the value array). Second row:
			// the default "<marker> consultations: <count>" (dim 1).
			Formatter: opts.FuncOpts(`function (params) { var p = params[0]; return p.value[2] + '<br/>' + p.marker + p.seriesName + ': ' + p.value[1]; }`),
		}),
		charts.WithLegendOpts(opts.Legend{Show: opts.Bool(false)}),
		charts.WithGridOpts(opts.Grid{Left: "0", Right: "0", Top: "50%", Bottom: "0", Show: opts.Bool(false), ContainLabel: opts.Bool(false)}),
		charts.WithXAxisOpts(opts.XAxis{SplitNumber: len(chartData), Show: opts.Bool(false), Type: "value"}),
		charts.WithYAxisOpts(opts.YAxis{Show: opts.Bool(false)}),
	)

	lineData := []opts.LineData{}

	for i, p := range chartData {
		// Value layout: [x index, count, date]. The axis only uses dims 0/1, so
		// the date in dim 2 is carried purely for the tooltip and leaves the
		// value-typed X axis (and the chart width) untouched.
		lineData = append(lineData, opts.LineData{Value: []interface{}{i, p.Value, p.Date}})
	}

	chart.AddSeries(
		"consultations",
		lineData,
		charts.WithAreaStyleOpts(opts.AreaStyle{Opacity: opts.Float(0.1)}),
		charts.WithLineChartOpts(opts.LineChart{
			Smooth: opts.Bool(true),
		}),
		charts.WithLabelOpts(opts.Label{
			Show: opts.Bool(false),
		}),
	)

	resizeJs := `
		let resizeTimeout_%MY_ECHARTS%;
		const handleResize_%MY_ECHARTS% = () => {
			clearTimeout(resizeTimeout_%MY_ECHARTS%);
			resizeTimeout_%MY_ECHARTS% = setTimeout(() => %MY_ECHARTS%.resize(), 100);
		};
		window.addEventListener('resize', handleResize_%MY_ECHARTS%);
		window.addEventListener('DOMContentLoaded', handleResize_%MY_ECHARTS%);
		setTimeout(() => %MY_ECHARTS%.resize(), 0);
	
	`

	chart.AddJSFuncStrs(opts.FuncOpts(resizeJs))

	snippet := chart.RenderSnippet()

	return fmt.Sprintf("%s %s", snippet.Element, snippet.Script)
}
