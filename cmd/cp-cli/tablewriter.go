package main

import (
	"github.com/olekukonko/tablewriter"
	"log"
	"os"
)

type VisualTable struct {
	Header   []string
	Data     [][]string
	RowColor []RowColor
}

type RowColor struct {
	row    int
	column []int
	color  []tablewriter.Colors
}

func NewVisualTable(header []string, data [][]string, rowColor []RowColor) *VisualTable {

	return &VisualTable{
		Header:   header,
		Data:     data,
		RowColor: rowColor,
	}
}

func (v *VisualTable) Generate() {
	table := tablewriter.NewWriter(os.Stdout)

	for _, rowColor := range v.RowColor {
		if len(v.Data) < rowColor.row {
			log.Println("index out of range: data < row")
			break
		}
		rowLength := len(v.Data[rowColor.row])
		var rowColors []tablewriter.Colors
		for i := 0; i < rowLength; i++ {
			var defaultFlag = true
			for n, colIndex := range rowColor.column {
				if i == colIndex {
					rowColors = append(rowColors, rowColor.color[n])
					defaultFlag = false
				}
			}
			if defaultFlag {
				rowColors = append(rowColors, tablewriter.Colors{})
			}
		}
		table.Rich(v.Data[rowColor.row], rowColors)
	}

	table.SetHeader(v.Header)
	table.SetAutoWrapText(false)
	table.SetAutoFormatHeaders(true)
	table.SetHeaderAlignment(tablewriter.ALIGN_LEFT)
	table.SetHeaderLine(false)
	table.SetAlignment(tablewriter.ALIGN_LEFT)
	table.SetCenterSeparator("")
	table.SetColumnSeparator("")
	table.SetRowSeparator("")
	table.SetBorder(false)
	table.SetTablePadding("\t")
	table.SetNoWhiteSpace(true)
	table.AppendBulk(v.Data)
	table.Render()
}

//func main() {
//	header := []string{"Date", "Description", "CV2", "Amount"}
//	data := [][]string{
//		[]string{"1/1/2014", "Domain name", "2233", "$10.98"},
//		[]string{"1/1/2014", "January Hosting", "2233", "$54.95"},
//		[]string{"1/4/2014", "February Hosting ", "2233", "$51.00"},
//		[]string{"1/4/2014", "February Extra Bandwidth", "2233", "$30.00"},
//	}
//
//	NewVisualTable(header, data, []RowColor{
//		{
//			row:    1,
//			column: []int{1, 3},
//			color:  []tablewriter.Colors{{tablewriter.Normal, tablewriter.FgRedColor}, {tablewriter.Bold, tablewriter.FgWhiteColor}},
//		},
//	}).Generate()
//}
