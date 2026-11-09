package sheet

import (
	"bytes"
	"testing"

	"github.com/xuri/excelize/v2"
)

func TestFormulasExportsAndUnsafeFunctions(t *testing.T) {
	book := Book{Tabs: []Tab{{Name: "Budget", Cells: map[string]Cell{"A1": {Value: "Item", Bold: true}, "B1": {Value: "Cost", Bold: true}, "B2": {Value: 12.5}, "B3": {Value: 7.5}, "B4": {Formula: "=SUM(B2:B3)", Format: "0.00"}, "A5": {Value: "=not a formula"}}}}}
	result, err := Calculate(book)
	if err != nil || result["Budget"]["B4"].Value != "20.00" {
		t.Fatal(err, result)
	}
	data, err := XLSX(book)
	if err != nil {
		t.Fatal(err)
	}
	file, err := excelize.OpenReader(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	formula, _ := file.GetCellFormula("Budget", "B4")
	if formula != "SUM(B2:B3)" {
		t.Fatal(formula)
	}
	csv, err := CSV(book, "Budget")
	if err != nil || !bytes.Contains(csv, []byte("'=not a formula")) {
		t.Fatal(err, string(csv))
	}
	tab := book.Tabs[0]
	tab.Cells["B4"] = Cell{Formula: `=WEBSERVICE("https://example.com")`}
	if Validate(book) == nil {
		t.Fatal("network formula accepted")
	}
}
