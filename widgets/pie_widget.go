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

type PieWidget struct{}

func (t PieWidget) Render(ctx context.Context, config Configuration, data Data) templ.Component {
	if errMsg := data.Error; errMsg != "" {
		return renderWidgetErrorTempl(errMsg)
	}

	if len(data.Rows) == 0 && len(data.Columns) != 0 {
		return renderNoData()
	}

	if len(data.Columns) == 0 {
		return renderSpinner()
	}

	chartHTML := t.generatePieChart(ctx, config, data)

	chartHTML = strings.Replace(chartHTML, `class="container"`, `style="height: 100%;"`, 1)

	return pieWidgetTempl(chartHTML)
}

func (t *PieWidget) generatePieChart(ctx context.Context, config Configuration, data Data) string {
	pie := charts.NewPie()
	seriesName := config.SeriesName

	theme := util.GetChartTheme(ctx)

	var title string

	pie.SetGlobalOptions(
		charts.WithTitleOpts(opts.Title{
			Title: title,
		}),
		charts.WithTooltipOpts(opts.Tooltip{Show: opts.Bool(true), Trigger: "item", Formatter: "{a} <br/>{b}: {c} ({d}%)"}),
		charts.WithLegendOpts(opts.Legend{Show: opts.Bool(true), Orient: "vertical", Left: "right"}),
		charts.WithInitializationOpts(opts.Initialization{
			Width:    "100%",
			Height:   "100%",
			Theme:    theme,
			Renderer: "svg",
		}),
	)

	pieData := map[string][]opts.PieData{}

	for _, row := range data.Rows {
		sn := seriesName
		if row["series"] != "" {
			sn = row["series"]
		}
		pieData[sn] = append(pieData[sn], opts.PieData{
			Name:  row["label"],
			Value: row["value"],
		})
	}

	for series, seriesData := range pieData {
		pie.AddSeries(series, seriesData,
			charts.WithPieChartOpts(opts.PieChart{
				Radius: []string{"40%", "70%"},
			}),
			charts.WithLabelOpts(opts.Label{
				Show:     opts.Bool(false),
				Position: "center",
			}),
			charts.WithEmphasisOpts(opts.Emphasis{
				Label: &opts.Label{
					Show:       opts.Bool(true),
					FontSize:   18,
					FontWeight: "bold",
				},
			}),
		)
	}

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

	pie.AddJSFuncStrs(opts.FuncOpts(resizeJs))

	snippet := pie.RenderSnippet()

	return fmt.Sprintf("%s %s", snippet.Element, snippet.Script)
}
