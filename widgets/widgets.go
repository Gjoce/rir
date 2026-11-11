package widgets

import (
	"context"

	"github.com/a-h/templ"
)

const (
	WidgetTypeText = "text"
	WidgetTypePie  = "pie"
	WidgetTypeBar  = "bar"
)

var Map = map[string]Widget{
	WidgetTypeText: TextWidget{},
	WidgetTypePie:  PieWidget{},
	WidgetTypeBar:  BarWidget{},
}

type Configuration struct {
	TextFontSize    int    `json:"text_font_size,omitempty"`
	SubtextFontSize int    `json:"sub_font_size,omitempty"`
	SeriesName      string `json:"series_name,omitempty"`
	Query           string `json:"query,omitempty"`
}

type Data struct {
	ID      string
	Error   string
	Columns []string
	Rows    []map[string]string
	Title   string
}

type Widget interface {
	Render(ctx context.Context, config Configuration, data Data) templ.Component
}
