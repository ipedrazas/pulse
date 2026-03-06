package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"
)

func printTable(headers []string, rows [][]string) {
	fprintTable(os.Stdout, headers, rows)
}

func printJSON(v any) error {
	return fprintJSON(os.Stdout, v)
}

func fprintTable(out io.Writer, headers []string, rows [][]string) {
	w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, strings.Join(headers, "\t"))
	for _, row := range rows {
		fmt.Fprintln(w, strings.Join(row, "\t"))
	}
	w.Flush()
}

func fprintJSON(out io.Writer, v any) error {
	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}
