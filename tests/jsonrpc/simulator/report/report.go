package report

import (
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/xuri/excelize/v2"

	"github.com/cosmos/evm/tests/jsonrpc/simulator/config"
	"github.com/cosmos/evm/tests/jsonrpc/simulator/types"
	"github.com/cosmos/evm/tests/jsonrpc/simulator/utils"
)

const totalWidth = 63

// Results prints or saves the RPC results based on the verbosity flag and output format
func Results(results []*types.RpcResult, verbose bool, outputExcel bool, rCtx ...*types.RPCContext) {
	summary := &types.TestSummary{}
	for _, result := range results {
		summary.AddResult(result)
	}
	if outputExcel {
		f := excelize.NewFile()
		name := fmt.Sprintf("geth%s", config.GethVersion)
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

	// Print dual API comparison summary if available
	if len(rCtx) > 0 && rCtx[0] != nil && rCtx[0].EnableComparison {
		PrintComparisonSummary(rCtx[0])
		PrintComparisonToRPCSummaryDiscrepancy(summary, rCtx[0])
	}
}

func PrintHeader() {
	line := strings.Repeat("═", totalWidth)
	fmt.Printf("\n%s\n", line)
	fmt.Println("           Cosmos EVM JSON-RPC Compatibility Test           ")
	fmt.Printf("%s\n", line)
}

// sortResultsByStatus sorts results by status priority: PASS, FAIL, NOT_IMPL, LEGACY, SKIP
func sortResultsByStatus(results []*types.RpcResult) {
	statusPriority := map[types.RpcStatus]int{
		types.Ok:             1, // PASS
		types.Error:          2, // FAIL
		types.NotImplemented: 3, // NOT_IMPL
		types.Legacy:         4, // LEGACY
		types.Skipped:        5, // SKIP
	}

	sort.Slice(results, func(i, j int) bool {
		return statusPriority[results[i].Status] < statusPriority[results[j].Status]
	})
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

	// Print each category with namespace-based names
	categoryOrder := []string{"web3", "net", "eth", "personal", "miner", "txpool", "debug", "engine", "trace", "admin", "les"}
	categoryDisplayNames := map[string]string{
		"web3":     "Web3",
		"net":      "Net",
		"eth":      "Ethereum",
		"personal": "Personal (Deprecated)",
		"miner":    "Miner (Deprecated)",
		"txpool":   "TxPool",
		"debug":    "Debug",
		"engine":   "Engine API",
		"trace":    "Trace",
		"admin":    "Admin",
		"les":      "LES (Light Ethereum Subprotocol)",
	}

	for _, categoryName := range categoryOrder {
		if results, exists := categories[categoryName]; exists {
			displayName := categoryDisplayNames[categoryName]
			if displayName == "" {
				displayName = categoryName
			}

			// Sort results by status priority within each category
			sortResultsByStatus(results)

			// Calculate padding for consistent width
			methodsText := fmt.Sprintf(" %s Methods ", displayName)
			padding := totalWidth - len(methodsText)
			leftPadding := padding / 2
			rightPadding := padding - leftPadding

			subtitle := fmt.Sprintf("\n%s%s%s",
				strings.Repeat("═", leftPadding),
				methodsText,
				strings.Repeat("═", rightPadding))
			color.Cyan(subtitle)
			for _, result := range results {
				ColorPrint(result, verbose)
			}
		}
	}

	// Print any uncategorized results
	if results, exists := categories["Uncategorized"]; exists {
		// Calculate padding for consistent width
		methodsText := " Uncategorized Methods "
		padding := totalWidth - len(methodsText)
		leftPadding := padding / 2
		rightPadding := padding - leftPadding

		subtitle := fmt.Sprintf("\n%s%s%s",
			strings.Repeat("═", leftPadding),
			methodsText,
			strings.Repeat("═", rightPadding))
		color.Cyan(subtitle)
		for _, result := range results {
			ColorPrint(result, verbose)
		}
	}
}

func PrintCategoryMatrix(summary *types.TestSummary) {
	line := strings.Repeat("═", totalWidth)
	fmt.Printf("\n%s\n", line)
	fmt.Println("                      CATEGORY SUMMARY                      ")
	fmt.Printf("%s\n", line)

	// Define the order of categories (by namespace)
	categoryOrder := []string{"web3", "net", "eth", "personal", "miner", "txpool", "debug", "engine", "admin", "les"}

	// Print header without subcategory column
	fmt.Printf("%-20s │ %s │ %s │ %s │ %s │ %s │ %s\n",
		"Category",
		color.GreenString("Pass"),
		color.RedString("Fail"),
		color.YellowString("N/Im"),
		color.BlueString("Lgcy"),
		color.HiBlackString("Skip"),
		color.HiWhiteString("Total"))

	fmt.Println("─────────────────────┼──────┼──────┼──────┼──────┼──────┼──────")

	// Print each category in the defined order
	for _, categoryName := range categoryOrder {
		if catSummary, exists := summary.Categories[categoryName]; exists && catSummary.Total > 0 {
			// Format counts with colors for non-zero values
			passColor := fmt.Sprintf("%4d", catSummary.Passed)
			if catSummary.Passed > 0 {
				passColor = color.GreenString("%4d", catSummary.Passed)
			}

			failColor := fmt.Sprintf("%4d", catSummary.Failed)
			if catSummary.Failed > 0 {
				failColor = color.RedString("%4d", catSummary.Failed)
			}

			nimplColor := fmt.Sprintf("%4d", catSummary.NotImplemented)
			if catSummary.NotImplemented > 0 {
				nimplColor = color.YellowString("%4d", catSummary.NotImplemented)
			}

			legacyColor := fmt.Sprintf("%4d", catSummary.Legacy)
			if catSummary.Legacy > 0 {
				legacyColor = color.BlueString("%4d", catSummary.Legacy)
			}

			skipColor := fmt.Sprintf("%4d", catSummary.Skipped)
			if catSummary.Skipped > 0 {
				skipColor = color.HiBlackString("%4d", catSummary.Skipped)
			}

			fmt.Printf("%-20s │ %s │ %s │ %s │ %s │ %s │ %5d\n",
				categoryName,
				passColor,
				failColor,
				nimplColor,
				legacyColor,
				skipColor,
				catSummary.Total)
		}
	}

	// Print any additional categories not in the predefined order
	predefinedCategories := make(map[string]bool)
	for _, cat := range categoryOrder {
		predefinedCategories[cat] = true
	}

	for categoryName, catSummary := range summary.Categories {
		if !predefinedCategories[categoryName] && catSummary.Total > 0 {
			// Format counts with colors for non-zero values
			passColor := fmt.Sprintf("%4d", catSummary.Passed)
			if catSummary.Passed > 0 {
				passColor = color.GreenString("%4d", catSummary.Passed)
			}

			failColor := fmt.Sprintf("%4d", catSummary.Failed)
			if catSummary.Failed > 0 {
				failColor = color.RedString("%4d", catSummary.Failed)
			}

			nimplColor := fmt.Sprintf("%4d", catSummary.NotImplemented)
			if catSummary.NotImplemented > 0 {
				nimplColor = color.YellowString("%4d", catSummary.NotImplemented)
			}

			legacyColor := fmt.Sprintf("%4d", catSummary.Legacy)
			if catSummary.Legacy > 0 {
				legacyColor = color.BlueString("%4d", catSummary.Legacy)
			}

			skipColor := fmt.Sprintf("%4d", catSummary.Skipped)
			if catSummary.Skipped > 0 {
				skipColor = color.HiBlackString("%4d", catSummary.Skipped)
			}

			fmt.Printf("%-20s │ %s │ %s │ %s │ %s │ %s │ %5d\n",
				categoryName,
				passColor,
				failColor,
				nimplColor,
				legacyColor,
				skipColor,
				catSummary.Total)
		}
	}
}

func PrintSummary(summary *types.TestSummary) {
	line := strings.Repeat("═", totalWidth)
	fmt.Printf("\n%s\n", line)
	fmt.Println("                     FINAL SUMMARY                     ")
	fmt.Printf("%s\n", line)

	color.Green("Passed:          %d", summary.Passed)
	color.Red("Failed:          %d", summary.Failed)
	color.Yellow("Not Implemented: %d", summary.NotImplemented)
	color.Blue("Legacy:          %d", summary.Legacy)
	color.HiBlack("Skipped:         %d", summary.Skipped)
	color.Cyan("Total:           %d", summary.Total)
}

func PrintComparisonSummary(rCtx *types.RPCContext) {
	summary := rCtx.GetComparisonSummary()
	if summary == nil || summary["total"] == 0 {
		return
	}

	line := strings.Repeat("═", totalWidth)
	fmt.Printf("\n%s\n", line)
	fmt.Println("                DUAL API STRUCTURE COMPARISON             ")
	fmt.Printf("%s\n", line)

	fmt.Printf("Total Comparisons:       %d\n", summary["total"])
	color.Green("Structure Matches:       %d", summary["structure_matches"])
	color.Cyan("Type Matches:            %d", summary["type_matches"])
	color.Blue("Error Consistency:       %d", summary["error_matches"])
	color.Yellow("Structural Differences:  %d", summary["differences"])

	// Calculate missing categories to explain the math
	structureNoMatch := summary["total"] - summary["structure_matches"] - summary["differences"]

	if structureNoMatch > 0 {
		color.HiBlack("Structure Unknown:       %d (connection failed/not comparable)", structureNoMatch)
	}

	PrintDetailedComparisonBreakdown(rCtx)

	// Calculate structure compatibility percentage
	if summary["total"] > 0 {
		structureCompatibilityPercent := float64(summary["structure_matches"]) / float64(summary["total"]) * 100
		typeCompatibilityPercent := float64(summary["type_matches"]) / float64(summary["total"]) * 100
		errorCompatibilityPercent := float64(summary["error_matches"]) / float64(summary["total"]) * 100

		fmt.Printf("\nCompatibility Scores:\n")
		if structureCompatibilityPercent >= 90 {
			color.Green("  Structure Compatibility: %.1f%%", structureCompatibilityPercent)
		} else if structureCompatibilityPercent >= 70 {
			color.Yellow("  Structure Compatibility: %.1f%%", structureCompatibilityPercent)
		} else {
			color.Red("  Structure Compatibility: %.1f%%", structureCompatibilityPercent)
		}

		if typeCompatibilityPercent >= 90 {
			color.Green("  Type Compatibility:      %.1f%%", typeCompatibilityPercent)
		} else if typeCompatibilityPercent >= 70 {
			color.Yellow("  Type Compatibility:      %.1f%%", typeCompatibilityPercent)
		} else {
			color.Red("  Type Compatibility:      %.1f%%", typeCompatibilityPercent)
		}

		if errorCompatibilityPercent >= 90 {
			color.Green("  Error Compatibility:     %.1f%%", errorCompatibilityPercent)
		} else if errorCompatibilityPercent >= 70 {
			color.Yellow("  Error Compatibility:     %.1f%%", errorCompatibilityPercent)
		} else {
			color.Red("  Error Compatibility:     %.1f%%", errorCompatibilityPercent)
		}
	}
}

func PrintDetailedComparisonBreakdown(rCtx *types.RPCContext) {
	// Categorize all comparison results
	var (
		structureMatches []string
		structureDiffs   []string
		typeMatches      []string
		typeMismatches   []string
		errorMatches     []string
		errorMismatches  []string
		unknown          []string
	)

	for _, result := range rCtx.ComparisonResults {
		methodName := result.Method

		if result.StructureMatch {
			structureMatches = append(structureMatches, methodName)
		} else if len(result.Differences) > 0 {
			structureDiffs = append(structureDiffs, methodName)
		} else {
			unknown = append(unknown, methodName)
		}

		if result.TypeMatch {
			typeMatches = append(typeMatches, methodName)
		} else {
			typeMismatches = append(typeMismatches, methodName)
		}

		if result.ErrorsMatch {
			errorMatches = append(errorMatches, methodName)
		} else {
			errorMismatches = append(errorMismatches, methodName)
		}
	}

	fmt.Printf("\n" + strings.Repeat("─", totalWidth) + "\n")
	fmt.Printf("DETAILED BREAKDOWN:\n")
	fmt.Printf(strings.Repeat("─", totalWidth) + "\n")

	// Structure analysis
	fmt.Printf("\n1. STRUCTURE ANALYSIS (Total: %d):\n", len(rCtx.ComparisonResults))
	color.Green("   ✓ Structure Matches (%d): %s", len(structureMatches), formatMethodList(structureMatches))
	color.Yellow("   ⚠ Structure Differences (%d): %s", len(structureDiffs), formatMethodList(structureDiffs))
	if len(unknown) > 0 {
		color.HiBlack("   ? Unknown/Failed (%d): %s", len(unknown), formatMethodList(unknown))
	}

	// Type analysis
	fmt.Printf("\n2. TYPE ANALYSIS (Total: %d):\n", len(rCtx.ComparisonResults))
	color.Green("   ✓ Type Matches (%d): %s", len(typeMatches), formatMethodList(typeMatches))
	color.Red("   ✗ Type Mismatches (%d): %s", len(typeMismatches), formatMethodList(typeMismatches))

	// Error analysis
	fmt.Printf("\n3. ERROR CONSISTENCY ANALYSIS (Total: %d):\n", len(rCtx.ComparisonResults))
	color.Green("   ✓ Error Consistent (%d): %s", len(errorMatches), formatMethodList(errorMatches))
	color.Red("   ✗ Error Inconsistent (%d): %s", len(errorMismatches), formatMethodList(errorMismatches))

	// Detailed issues for type mismatches
	if len(typeMismatches) > 0 {
		fmt.Printf("\n" + strings.Repeat("─", 40) + "\n")
		fmt.Printf("TYPE MISMATCH DETAILS:\n")
		fmt.Printf(strings.Repeat("─", 40) + "\n")
		for _, result := range rCtx.ComparisonResults {
			if !result.TypeMatch {
				fmt.Printf("  • %s:\n", result.Method)
				fmt.Printf("    EVMD Type: %s\n", result.EvmdType)
				fmt.Printf("    Geth Type: %s\n", result.GethType)
				if result.EvmdError != "" || result.GethError != "" {
					fmt.Printf("    EVMD Error: %s\n", result.EvmdError)
					fmt.Printf("    Geth Error: %s\n", result.GethError)
				}
				fmt.Println()
			}
		}
	}

	// Detailed issues for error mismatches
	if len(errorMismatches) > 0 {
		fmt.Printf("\n" + strings.Repeat("─", 40) + "\n")
		fmt.Printf("ERROR INCONSISTENCY DETAILS:\n")
		fmt.Printf(strings.Repeat("─", 40) + "\n")
		for _, result := range rCtx.ComparisonResults {
			if !result.ErrorsMatch {
				fmt.Printf("  • %s:\n", result.Method)
				fmt.Printf("    EVMD Error: %s\n", result.EvmdError)
				fmt.Printf("    Geth Error: %s\n", result.GethError)
				fmt.Println()
			}
		}
	}

	// Structure difference details
	if len(structureDiffs) > 0 {
		fmt.Printf("\n" + strings.Repeat("─", 40) + "\n")
		fmt.Printf("STRUCTURAL DIFFERENCE DETAILS:\n")
		fmt.Printf(strings.Repeat("─", 40) + "\n")
		for _, result := range rCtx.ComparisonResults {
			if len(result.Differences) > 0 {
				fmt.Printf("  • %s:\n", result.Method)
				for _, diff := range result.Differences {
					if strings.Contains(diff, "request failed") {
						color.Cyan("    - %s (connection issue)", diff)
					} else {
						fmt.Printf("    - %s\n", diff)
					}
				}
				fmt.Println()
			}
		}
	}
}

func PrintComparisonToRPCSummaryDiscrepancy(testSummary *types.TestSummary, rCtx *types.RPCContext) {
	comparisonSummary := rCtx.GetComparisonSummary()
	if comparisonSummary == nil {
		return
	}

	line := strings.Repeat("═", totalWidth)
	fmt.Printf("\n%s\n", line)
	fmt.Println("             COUNT DISCREPANCY ANALYSIS                 ")
	fmt.Printf("%s\n", line)

	// Find eth category in test results
	ethCategory := testSummary.Categories["eth"]
	if ethCategory == nil {
		fmt.Println("No eth category found in test summary")
		return
	}

	totalEthAPIs := ethCategory.Total
	totalComparisons := comparisonSummary["total"]

	fmt.Printf("Total Eth APIs (Category Summary):  %d\n", totalEthAPIs)
	fmt.Printf("Total Comparisons (Dual API):       %d\n", totalComparisons)

	// Analyze the structure match math
	fmt.Printf("\nStructure Match Math Analysis:\n")
	fmt.Printf("  Structure Matches:     %d\n", comparisonSummary["structure_matches"])
	fmt.Printf("  Structure Differences: %d\n", comparisonSummary["differences"])
	structureUnknown := totalComparisons - comparisonSummary["structure_matches"] - comparisonSummary["differences"]
	fmt.Printf("  Structure Unknown:     %d\n", structureUnknown)
	fmt.Printf("  Total:                 %d\n", totalComparisons)

	if structureUnknown > 0 {
		color.Yellow("Note: %d APIs have unknown structure status (likely connection/request failures)", structureUnknown)
	}

	// Show breakdown by status
	fmt.Printf("\nType & Error Match Details:\n")
	typeMatches := comparisonSummary["type_matches"]
	typeMismatches := totalComparisons - typeMatches
	errorMatches := comparisonSummary["error_matches"]
	errorMismatches := totalComparisons - errorMatches

	fmt.Printf("  Type Matches: %d, Mismatches: %d\n", typeMatches, typeMismatches)
	fmt.Printf("  Error Matches: %d, Mismatches: %d\n", errorMatches, errorMismatches)
}

func formatMethodList(methods []string) string {
	if len(methods) == 0 {
		return "(none)"
	}
	if len(methods) <= 5 {
		return strings.Join(methods, ", ")
	}
	return fmt.Sprintf("%s, ... and %d more", strings.Join(methods[:5], ", "), len(methods)-5)
}

func ColorPrint(result *types.RpcResult, verbose bool) {
	method := result.Method
	status := result.Status

	// Include description if it exists (helps distinguish multiple tests with same method name)
	methodDisplay := string(method)
	if result.Description != "" {
		methodDisplay = fmt.Sprintf("%s (%s)", method, result.Description)
	}

	switch status {
	case types.Ok:
		value := result.Value
		if !verbose {
			value = ""
		}
		color.Green("[%s] %s", status, methodDisplay)
		if verbose && value != nil {
			fmt.Printf(" - %v", value)
		}
	case types.Legacy:
		color.Blue("[%s] %s", status, methodDisplay)
		if verbose && result.ErrMsg != "" {
			fmt.Printf(" - %s", result.ErrMsg)
		}
	case types.NotImplemented:
		color.Yellow("[%s] %s", status, methodDisplay)
	case types.Skipped:
		color.HiBlack("[%s] %s", status, methodDisplay)
		if verbose && result.ErrMsg != "" {
			fmt.Printf(" - %s", result.ErrMsg)
		}
	case types.Error:
		color.Red("[%s] %s", status, methodDisplay)
		if verbose && result.ErrMsg != "" {
			fmt.Printf(" - %s", result.ErrMsg)
		}
	}
}
