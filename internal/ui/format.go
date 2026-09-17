package ui

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	headerStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("12"))
	boldStyle   = lipgloss.NewStyle().Bold(true)
	dimStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	greenStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
	yellowStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("11"))
	redStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
	cyanStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("14"))
)

// PrintStep prints an aligned step message like:
//
//	setup:    cloning node_modules... done
func PrintStep(step, message string) {
	stepFormatted := fmt.Sprintf("%-10s", step+":")
	fmt.Printf("  %s %s\n", headerStyle.Render(stepFormatted), message)
}

// PrintSubStep prints an indented sub-item like:
//
//	postgres → 127.0.0.1:55123
func PrintSubStep(message string) {
	fmt.Printf("             %s\n", message)
}

// PrintSuccess prints a green success line.
func PrintSuccess(msg string) {
	fmt.Println(greenStyle.Render(msg))
}

// PrintWarning prints a yellow warning line.
func PrintWarning(msg string) {
	fmt.Println(yellowStyle.Render("Warning: " + msg))
}

// PrintError prints a red error line.
func PrintError(msg string) {
	fmt.Fprintf(os.Stderr, "%s\n", redStyle.Render("Error: "+msg))
}

// RenderTable renders an aligned table.
func RenderTable(w io.Writer, headers []string, rows [][]string) {
	if len(rows) == 0 {
		return
	}

	colWidths := make([]int, len(headers))
	for i, h := range headers {
		colWidths[i] = len(h)
	}

	for _, row := range rows {
		for i, val := range row {
			if i < len(colWidths) && len(val) > colWidths[i] {
				colWidths[i] = len(val)
			}
		}
	}

	// Print headers
	var hParts []string
	for i, h := range headers {
		padding := colWidths[i] - len(h)
		hParts = append(hParts, boldStyle.Render(h)+strings.Repeat(" ", padding))
	}
	fmt.Fprintln(w, strings.Join(hParts, "   "))

	// Print rows
	for _, row := range rows {
		var rParts []string
		for i, cell := range row {
			padding := 0
			if i < len(colWidths) {
				padding = colWidths[i] - len(cell)
				if padding < 0 {
					padding = 0
				}
			}
			rParts = append(rParts, cell+strings.Repeat(" ", padding))
		}
		fmt.Fprintln(w, strings.Join(rParts, "   "))
	}
}
