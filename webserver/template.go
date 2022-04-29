package webserver

import (
	"fmt"
	"html/template"
	"os"
)

// createTemplate takes in a path to a template file and parses the file to create a Template instance.
func createTemplate(templatePath string) (*template.Template, error) {
	if _, err := os.Stat(templatePath); err != nil {
		return nil, fmt.Errorf("Error opening template file: %v", err)
	}
	tmpl, err := template.New("index").
		Funcs(template.FuncMap{
			"filter":          filter,
			"commitLink":      commitLink,
			"formattedTime":   formattedTime,
			"latency":         latency,
			"appliedRecently": appliedRecently,
			"status":          status,
		}).
		ParseFiles(templatePath)
	if err != nil {
		return nil, fmt.Errorf("Error parsing template: %v", err)
	}
	return tmpl, nil
}
