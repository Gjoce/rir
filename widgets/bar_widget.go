package widgets

import (
	"context"
	"cybermedico/cmd/cybermedico/api/util"
	"fmt"
	"strings"

	"github.com/a-h/templ"
	"github.com/go-echarts/go-echarts/v2/charts"
	"github.com/go-echarts/go-echarts/v2/opts"
)

type BarWidget struct{}

func (b BarWidget) Render(ctx context.Context, config Configuration, data Data) templ.Component {
	if errMsg := data.Error; errMsg != "" {
		return renderWidgetErrorTempl(errMsg)
	}

	if len(data.Rows) == 0 && len(data.Columns) != 0 {
		return renderNoData()
	}

	if len(data.Columns) == 0 {
		return renderSpinner()
	}

	chartHTML := b.generateBarChart(ctx, config, data)

	chartHTML = strings.Replace(chartHTML, `class="container"`, `style="height: 100%;"`, 1)

	return barWidgetTempl(chartHTML)
}

// generateBarChart renders a horizontal bar chart from {label, value} rows. The
// query supplies one row per bar; rows should be ordered ascending by value so
// the largest bar lands at the top (ECharts category index 0 is the bottom).
func (b *BarWidget) generateBarChart(ctx context.Context, config Configuration, data Data) string {
	bar := charts.NewBar()

	theme := util.GetChartTheme(ctx)

	bar.SetGlobalOptions(
		charts.WithInitializationOpts(opts.Initialization{
			Width:    "100%",
			Height:   "100%",
			Theme:    theme,
			Renderer: "svg",
		}),
		charts.WithTooltipOpts(opts.Tooltip{Show: opts.Bool(true), Trigger: "axis"}),
		charts.WithLegendOpts(opts.Legend{Show: opts.Bool(false)}),
		// ContainLabel keeps long category labels (e.g. doctor names) inside the canvas.
		charts.WithGridOpts(opts.Grid{Left: "3%", Right: "6%", Top: "10%", Bottom: "3%", ContainLabel: opts.Bool(true)}),
		charts.WithXAxisOpts(opts.XAxis{Type: "value"}),
		charts.WithYAxisOpts(opts.YAxis{Type: "category"}),
	)

	labels := make([]string, 0, len(data.Rows))
	barData := make([]opts.BarData, 0, len(data.Rows))
	for _, row := range data.Rows {
		labels = append(labels, row["label"])
		barData = append(barData, opts.BarData{Value: row["value"]})
	}

	bar.SetXAxis(labels).
		AddSeries(config.SeriesName, barData,
			charts.WithLabelOpts(opts.Label{Show: opts.Bool(true), Position: "right"}),
		).
		// XYReversal turns the vertical bar into a horizontal one by moving the
		// category data onto the Y axis.
		XYReversal()

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

	bar.AddJSFuncStrs(opts.FuncOpts(resizeJs))

	snippet := bar.RenderSnippet()

	return fmt.Sprintf("%s %s", snippet.Element, snippet.Script)
}
