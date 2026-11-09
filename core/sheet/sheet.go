package sheet

import (
	"bytes"
	"encoding/csv"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/xuri/excelize/v2"
)

type Cell struct {
	Value   any    `json:"value,omitempty"`
	Formula string `json:"formula,omitempty"`
	Bold    bool   `json:"bold,omitempty"`
	Fill    string `json:"fill,omitempty"`
	Format  string `json:"format,omitempty"`
}
type Tab struct {
	Name  string          `json:"name"`
	Cells map[string]Cell `json:"cells"`
}
type Book struct {
	Tabs []Tab `json:"tabs"`
}
type Display struct {
	Value string `json:"value"`
	Error string `json:"error,omitempty"`
}

var function = regexp.MustCompile(`(?i)([a-z_][a-z0-9_.]*)\s*\(`)
var color = regexp.MustCompile(`^[0-9a-fA-F]{6}$`)

func Validate(book Book) error {
	if len(book.Tabs) < 1 || len(book.Tabs) > 20 {
		return errors.New("a workbook needs 1–20 tabs")
	}
	seen := map[string]bool{}
	count := 0
	allowed := map[string]bool{}
	for _, name := range strings.Fields("SUM AVERAGE MIN MAX COUNT COUNTA COUNTBLANK COUNTIF COUNTIFS SUMIF SUMIFS IF IFERROR AND OR NOT ROUND ROUNDUP ROUNDDOWN ABS INT MOD PRODUCT SUBTOTAL CONCAT CONCATENATE LEFT RIGHT MID LEN TRIM UPPER LOWER TEXT VALUE DATE YEAR MONTH DAY INDEX MATCH VLOOKUP HLOOKUP XLOOKUP SQRT POWER CEILING FLOOR") {
		allowed[name] = true
	}
	for _, tab := range book.Tabs {
		name := strings.TrimSpace(tab.Name)
		if name == "" || len([]rune(name)) > 31 || strings.ContainsAny(name, `[]:*?/\\`) || strings.HasPrefix(name, "'") || strings.HasSuffix(name, "'") || seen[strings.ToLower(name)] {
			return errors.New("tab names must be unique and valid Excel sheet names")
		}
		seen[strings.ToLower(name)] = true
		for address, cell := range tab.Cells {
			count++
			if count > 10000 {
				return errors.New("workbook exceeds 10000 populated cells")
			}
			column, row, err := excelize.CellNameToCoordinates(address)
			if err != nil || column > 100 || row > 1000 || address != strings.ToUpper(address) {
				return fmt.Errorf("cell %s is outside A1:CV1000", address)
			}
			switch value := cell.Value.(type) {
			case nil, bool, float64, int, int64:
			case string:
				if len(value) > 10000 {
					return errors.New("cell text exceeds 10000 bytes")
				}
			default:
				return errors.New("cell value must be text, number or boolean")
			}
			if len(cell.Formula) > 2000 || strings.ContainsAny(cell.Formula, "[]{}\r\n") {
				return errors.New("external, array, or oversized formulas are not supported")
			}
			for _, match := range function.FindAllStringSubmatch(cell.Formula, -1) {
				if !allowed[strings.ToUpper(match[1])] {
					return fmt.Errorf("unsupported formula function %s", match[1])
				}
			}
			if cell.Fill != "" && !color.MatchString(cell.Fill) {
				return errors.New("fill must be a six-digit RGB color")
			}
			if len(cell.Format) > 100 {
				return errors.New("number format is too long")
			}
		}
	}
	return nil
}
func workbook(book Book) (*excelize.File, error) {
	if err := Validate(book); err != nil {
		return nil, err
	}
	file := excelize.NewFile()
	fail := func(err error) (*excelize.File, error) { file.Close(); return nil, err }
	for index, tab := range book.Tabs {
		if index == 0 {
			if err := file.SetSheetName("Sheet1", tab.Name); err != nil {
				return fail(err)
			}
		} else {
			if _, err := file.NewSheet(tab.Name); err != nil {
				return fail(err)
			}
		}
		addresses := make([]string, 0, len(tab.Cells))
		for address := range tab.Cells {
			addresses = append(addresses, address)
		}
		sort.Strings(addresses)
		for _, address := range addresses {
			cell := tab.Cells[address]
			var err error
			if cell.Formula != "" {
				err = file.SetCellFormula(tab.Name, address, strings.TrimPrefix(cell.Formula, "="))
			} else {
				err = file.SetCellValue(tab.Name, address, cell.Value)
			}
			if err != nil {
				return fail(err)
			}
			if cell.Bold || cell.Fill != "" || cell.Format != "" {
				style := &excelize.Style{Font: &excelize.Font{Bold: cell.Bold}}
				if cell.Fill != "" {
					style.Fill = excelize.Fill{Type: "pattern", Color: []string{cell.Fill}, Pattern: 1}
				}
				if cell.Format != "" {
					style.CustomNumFmt = &cell.Format
				}
				id, err := file.NewStyle(style)
				if err != nil {
					return fail(err)
				}
				if err = file.SetCellStyle(tab.Name, address, address, id); err != nil {
					return fail(err)
				}
			}
		}
		file.SetColWidth(tab.Name, "A", "CV", 16)
	}
	return file, nil
}
func Calculate(book Book) (map[string]map[string]Display, error) {
	file, err := workbook(book)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	result := map[string]map[string]Display{}
	for _, tab := range book.Tabs {
		cells := map[string]Display{}
		for address, cell := range tab.Cells {
			display := Display{}
			if cell.Formula != "" {
				display.Value, err = file.CalcCellValue(tab.Name, address, excelize.Options{MaxCalcIterations: 100})
				if err != nil {
					display.Error = "Formula could not be calculated"
					if display.Value == "" {
						display.Value = "#ERROR!"
					}
				}
			} else {
				display.Value, _ = file.GetCellValue(tab.Name, address)
			}
			cells[address] = display
		}
		result[tab.Name] = cells
	}
	return result, nil
}
func XLSX(book Book) ([]byte, error) {
	file, err := workbook(book)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	buffer, err := file.WriteToBuffer()
	if err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}
func CSV(book Book, tabName string) ([]byte, error) {
	values, err := Calculate(book)
	if err != nil {
		return nil, err
	}
	var tab *Tab
	for i := range book.Tabs {
		if book.Tabs[i].Name == tabName {
			tab = &book.Tabs[i]
		}
	}
	if tab == nil {
		return nil, errors.New("tab does not exist")
	}
	width, height := 0, 0
	for address := range tab.Cells {
		column, row, _ := excelize.CellNameToCoordinates(address)
		width = max(width, column)
		height = max(height, row)
	}
	var output bytes.Buffer
	writer := csv.NewWriter(&output)
	for row := 1; row <= height; row++ {
		record := make([]string, width)
		for col := 1; col <= width; col++ {
			address, _ := excelize.CoordinatesToCellName(col, row)
			value := values[tabName][address].Value
			if _, ok := tab.Cells[address].Value.(string); ok && len(value) > 0 && strings.ContainsRune("=+-@\t\r", rune(value[0])) {
				value = "'" + value
			}
			record[col-1] = value
		}
		if err = writer.Write(record); err != nil {
			return nil, err
		}
	}
	writer.Flush()
	return output.Bytes(), writer.Error()
}
func Input(value string) Cell {
	if strings.HasPrefix(value, "=") {
		return Cell{Formula: value}
	}
	if number, err := strconv.ParseFloat(value, 64); err == nil && strings.TrimSpace(value) != "" {
		return Cell{Value: number}
	}
	return Cell{Value: value}
}
