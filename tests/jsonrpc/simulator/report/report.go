package report

import (
	"fmt"
	"log"
	"time"

	"github.com/fatih/color"
	"github.com/xuri/excelize/v2"

	"github.com/cosmos/evm/tests/jsonrpc/simulator/rpc"
	"github.com/cosmos/evm/tests/jsonrpc/simulator/types"
	"github.com/cosmos/evm/tests/jsonrpc/simulator/utils"
)

// ReportResults prints or saves the RPC results based on the verbosity flag and output format
func ReportResults(results []*types.RpcResult, verbose bool, outputExcel bool) {
	summary := &types.TestSummary{}
	for _, result := range results {
		summary.AddResult(result)
	}
	if outputExcel {
		f := excelize.NewFile()
		name := fmt.Sprintf("geth%s", rpc.GethVersion)
		if err := f.SetSheetName("Sheet1", name); err != nil {
			log.Fatalf("Failed to set sheet name: %v", err)
		}

		// set header
		header := []string{"Method", "Status", "Value", "Warnings", "ErrMsg"}
		for col, h := range header {
			cell := fmt.Sprintf("%s1", string(rune('A'+col)))
			if err := f.SetCellValue(name, cell, h); err != nil {
				log.Fatalf("Failed to set cell value: %v", err)
			}
		}

		// set columns width
		if err := f.SetColWidth(name, "A", "A", 30); err != nil {
			log.Fatalf("Failed to set col width: %v", err)
		}
		if err := f.SetColWidth(name, "C", "C", 40); err != nil {
			log.Fatalf("Failed to set col width: %v", err)
		}
		if err := f.SetColWidth(name, "E", "E", 40); err != nil {
			log.Fatalf("Failed to set col width: %v", err)
		}

		// set style for method column
		methodColStyle, err := f.NewStyle(&excelize.Style{
			Alignment: &excelize.Alignment{Vertical: "center"},
		})
		if err != nil {
			log.Fatalf("Failed to create style: %v", err)
		}
		if err = f.SetColStyle(name, "A", methodColStyle); err != nil {
			log.Fatalf("Failed to set col style: %v", err)
		}

		// set style for value column
		valueColStyle, err := f.NewStyle(&excelize.Style{
			Alignment: &excelize.Alignment{
				WrapText:   false,
				Horizontal: "left",
			},
		})
		if err != nil {
			log.Fatalf("Failed to create style: %v", err)
		}
		if err = f.SetColStyle(name, "C", valueColStyle); err != nil {
			log.Fatalf("Failed to set col style: %v", err)
		}

		fontStyle := &excelize.Style{Font: &excelize.Font{Bold: true}}
		for i, result := range results {
			row := i + 2
			warnings := "[]" // Empty warnings array for Excel compatibility
			methodCell := fmt.Sprintf("A%d", row)
			if err = f.SetCellValue(name, methodCell, result.Method); err != nil {
				log.Fatalf("Failed to set cell value: %v", err)
			}
			statusCell := fmt.Sprintf("B%d", row)
			if err = f.SetCellValue(name, statusCell, result.Status); err != nil {
				log.Fatalf("Failed to set cell value: %v", err)
			}
			valueCell := fmt.Sprintf("C%d", row)
			if err = f.SetCellValue(name, valueCell, result.Value); err != nil {
				log.Fatalf("Failed to set cell value: %v", err)
			}
			warningsCell := fmt.Sprintf("D%d", row)
			if err = f.SetCellValue(name, warningsCell, warnings); err != nil {
				log.Fatalf("Failed to set cell value: %v", err)
			}
			errCell := fmt.Sprintf("E%d", row)
			if err = f.SetCellValue(name, errCell, result.ErrMsg); err != nil {
				log.Fatalf("Failed to set cell value: %v", err)
			}

			// SET STYLES
			// set status column style based on status
			switch result.Status {
			case types.Ok:
				fontStyle.Font.Color = utils.GREEN
				s, err := f.NewStyle(fontStyle)
				if err != nil {
					log.Fatalf("Failed to create style: %v", err)
				}
				if err = f.SetCellStyle(name, statusCell, statusCell, s); err != nil {
					log.Fatalf("Failed to set cell style: %v", err)
				}
			case types.Error:
				fontStyle.Font.Color = utils.RED
				s, err := f.NewStyle(fontStyle)
				if err != nil {
					log.Fatalf("Failed to create style: %v", err)
				}
				if err = f.SetCellStyle(name, statusCell, statusCell, s); err != nil {
					log.Fatalf("Failed to set cell style: %v", err)
				}
			}

			if err = f.SetRowHeight(name, row, 20); err != nil {
				log.Fatalf("Failed to set row height: %v", err)
			}
		}
		// Set header style at last to avoid override by other styles
		headerStyle, err := f.NewStyle(&excelize.Style{
			Fill: excelize.Fill{Type: "pattern", Pattern: 1, Color: []string{"#D3D3D3"}},
			Font: &excelize.Font{Bold: true},
		})
		if err != nil {
			log.Fatalf("Failed to create style: %v", err)
		}
		if err = f.SetRowStyle(name, 1, 1, headerStyle); err != nil {
			log.Fatalf("Failed to set row style: %v", err)
		}

		fileName := fmt.Sprintf("rpc_results_%s.xlsx", time.Now().Format("15:04:05"))
		if err := f.SaveAs(fileName); err != nil {
			log.Fatalf("Failed to save Excel file: %v", err)
		}
		fmt.Println("Results saved to " + fileName)
	}

	PrintHeader()
	PrintCategorizedResults(results, verbose)
	PrintCategoryMatrix(summary)
	PrintSummary(summary)
}

func PrintHeader() {
	fmt.Println(`
══════════════════════════════════════════════
    Cosmos EVM JSON-RPC Compatibility Test
══════════════════════════════════════════════`)
}

func PrintCategorizedResults(results []*types.RpcResult, verbose bool) {
	categories := make(map[string][]*types.RpcResult)

	// Group results by category
	for _, result := range results {
		category := result.Category
		if category == "" {
			category = "Uncategorized"
		}
		categories[category] = append(categories[category], result)
	}

	// Print each category
	categoryOrder := []string{"Web3", "Net", "Core Eth", "Account & State", "Block", "Transaction", "Filter", "Mining", "EIP-1559", "Personal", "TxPool", "Debug", "Cosmos Extensions", "Engine API", "Not Implemented", "Uncategorized"}

	for _, categoryName := range categoryOrder {
		if results, exists := categories[categoryName]; exists {
			color.Cyan("\n=== %s Methods ===", categoryName)
			for _, result := range results {
				ColorPrint(result, verbose)
			}
		}
	}
}

func PrintCategoryMatrix(summary *types.TestSummary) {
	fmt.Println(`
═══════════════════════════════════════════════
                CATEGORY SUMMARY
═══════════════════════════════════════════════`)

	// Define the order of categories (by namespace)
	categoryOrder := []string{"web3", "net", "eth", "personal", "miner", "txpool", "debug", "engine", "trace", "admin"}

	// Print header with subcategory column
	fmt.Printf("%-15s │ %-15s │ %s │ %s │ %s │ %s │ %s │ %s\n",
		"Category",
		"Sub Category",
		color.GreenString("Pass"),
		color.RedString("Fail"),
		color.MagentaString("Depr"),
		color.YellowString("N/Im"),
		color.HiBlackString("Skip"),
		color.CyanString("Total"))

	fmt.Println("────────────────┼─────────────────┼──────┼──────┼──────┼──────┼──────┼──────")

	// Print each category and its subcategories
	for _, categoryName := range categoryOrder {
		if catSummary, exists := summary.Categories[categoryName]; exists {
			// Print category total first
			fmt.Printf("%-15s │ %-15s │ %4d │ %4d │ %4d │ %4d │ %4d │ %5d\n",
				categoryName,
				"All",
				catSummary.Passed,
				catSummary.Failed,
				catSummary.Deprecated,
				catSummary.NotImplemented,
				catSummary.Skipped,
				catSummary.Total)

			// Print subcategories if they exist
			if subcats, hasSubcats := summary.Subcategories[categoryName]; hasSubcats {
				// Define order for eth subcategories
				var subcatOrder []string
				if categoryName == "eth" {
					subcatOrder = []string{"Core", "EIP-1559", "Account & State", "Block", "Transaction", "Filter", "Mining", "Other"}
				} else {
					// For other categories, collect all subcategories
					for subName := range subcats {
						subcatOrder = append(subcatOrder, subName)
					}
				}

				for _, subName := range subcatOrder {
					if subSummary, exists := subcats[subName]; exists && subSummary.Total > 0 {
						fmt.Printf("%-15s │ %-15s │ %4d │ %4d │ %4d │ %4d │ %4d │ %5d\n",
							"",
							subName,
							subSummary.Passed,
							subSummary.Failed,
							subSummary.Deprecated,
							subSummary.NotImplemented,
							subSummary.Skipped,
							subSummary.Total)
					}
				}
			}
		}
	}

	// Print any uncategorized
	if catSummary, exists := summary.Categories["Uncategorized"]; exists {
		fmt.Printf("%-15s │ %-15s │ %4d │ %4d │ %4d │ %4d │ %4d │ %5d\n",
			"Uncategorized",
			"All",
			catSummary.Passed,
			catSummary.Failed,
			catSummary.Deprecated,
			catSummary.NotImplemented,
			catSummary.Skipped,
			catSummary.Total)
	}
}

func PrintSummary(summary *types.TestSummary) {
	fmt.Println(`
═══════════════════════════════════════════════
                   FINAL SUMMARY
═══════════════════════════════════════════════`)

	color.Green("Passed:          %d", summary.Passed)
	color.Red("Failed:          %d", summary.Failed)
	color.Magenta("Deprecated:      %d", summary.Deprecated)
	color.Yellow("Not Implemented: %d", summary.NotImplemented)
	color.HiBlack("Skipped:         %d", summary.Skipped)
	color.Cyan("Total:           %d", summary.Total)

	if summary.Failed > 0 {
		fmt.Printf("\n")
		color.Red("❌ Some tests failed. Check the detailed results above.")
	} else {
		fmt.Printf("\n")
		color.Green("✅ All implemented methods are working correctly!")
	}
}

func ColorPrint(result *types.RpcResult, verbose bool) {
	method := result.Method
	status := result.Status
	subcategory := ""
	if result.Subcategory != "" {
		subcategory = fmt.Sprintf(" (%s)", result.Subcategory)
	}

	switch status {
	case types.Ok:
		value := result.Value
		if !verbose {
			value = ""
		}
		color.Green("[%s] %s%s", status, method, subcategory)
		if verbose && value != nil {
			fmt.Printf(" - %v", value)
		}
		fmt.Println()
	case types.Deprecated:
		color.Magenta("[%s] %s%s - Method is deprecated but implemented", status, method, subcategory)
	case types.NotImplemented:
		color.Yellow("[%s] %s%s - Expected to be not implemented", status, method, subcategory)
	case types.Skipped:
		color.HiBlack("[%s] %s%s", status, method, subcategory)
		if result.ErrMsg != "" {
			fmt.Printf(" - %s", result.ErrMsg)
		}
		fmt.Println()
	case types.Error:
		color.Red("[%s] %s%s", status, method, subcategory)
		if result.ErrMsg != "" {
			fmt.Printf(" - %s", result.ErrMsg)
		}
		fmt.Println()
	}
}
