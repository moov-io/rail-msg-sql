package search

import (
	"cmp"
	"fmt"
	"html/template"
	"io"
	"path"
	"time"

	"github.com/moov-io/rail-msg-sql/webui"
)

var (
	indexTemplate = initTemplate("index", "index.html.tmpl")
)

type indexData struct {
	BaseURL string

	TimeRangeMin, TimeRangeMax time.Time
}

func baseURL(basePath string) string {
	cleaned := path.Clean(basePath)
	if cleaned == "." {
		return "/"
	}
	return cleaned + "/"
}

var templateFuncs template.FuncMap = map[string]interface{}{
	"yyyymmdd": func(when time.Time) string {
		return when.Format("2006-01-02")
	},
	"startDateParam": func(end time.Time) string {
		start := end.Add(-7 * 24 * time.Hour)
		return fmt.Sprintf("?startDate=%s&endDate=%s", start.Format("2006-01-02"), end.Format("2006-01-02"))
	},
	"endDateParam": func(start time.Time) string {
		end := start.Add(7 * 24 * time.Hour)
		return fmt.Sprintf("?startDate=%s&endDate=%s", start.Format("2006-01-02"), end.Format("2006-01-02"))
	},
}

func initTemplate(name, path string) *template.Template {
	fd, err := webui.WebRoot.Open(path)
	if err != nil {
		panic(fmt.Sprintf("error opening %s: %v", path, err)) //nolint:forbidigo
	}
	defer fd.Close()

	bs, err := io.ReadAll(fd)
	if err != nil {
		var filename string
		info, _ := fd.Stat()
		if info != nil {
			filename = info.Name()
		}
		filename = cmp.Or(filename, path)

		panic(fmt.Sprintf("error reading %s: %v", filename, err)) //nolint:forbidigo
	}

	return template.Must(template.New(name).Funcs(templateFuncs).Parse(string(bs)))
}
