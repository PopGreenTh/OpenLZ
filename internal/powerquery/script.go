package powerquery

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// GenerateParquetMScript produces Power Query M-code to load a local Parquet file into Power BI or Excel.
func GenerateParquetMScript(parquetFilePath string) string {
	escapedPath := strings.ReplaceAll(filepath.Clean(parquetFilePath), `\`, `\\`)
	return fmt.Sprintf(`// OpenLZ Power Query M-Formula for Power BI Desktop & Excel
// 1. Open Power BI Desktop or Excel -> Data -> Get Data -> Blank Query
// 2. Open "Advanced Editor" and replace existing code with this snippet:
let
    Source = Parquet.Document(File.Contents("%s")),
    #"Promoted Headers" = Table.PromoteHeaders(Source, [PromoteAllScalars=true]),
    #"Changed Type" = Table.TransformColumnTypes(#"Promoted Headers",{
        {"account_id", type text},
        {"account_name", type text},
        {"environment", type text},
        {"business_unit", type text},
        {"owner", type text},
        {"cost_center", type text},
        {"service", type text},
        {"resource_or_key", type text},
        {"metric", type text},
        {"actual_value", type number},
        {"expected_value", type number},
        {"potential_savings", type number},
        {"severity", type text},
        {"action_recommended", type text}
    })
in
    #"Changed Type"
`, escapedPath)
}

// GenerateWebMScript produces Power Query M-code to query the live OpenLZ REST feed.
func GenerateWebMScript(endpointURL string) string {
	return fmt.Sprintf(`// OpenLZ Power Query M-Formula for Live Web Data Refresh
// 1. Open Power BI Desktop or Excel -> Data -> Get Data -> Blank Query
// 2. Open "Advanced Editor" and replace existing code with this snippet:
let
    Source = Json.Document(Web.Contents("%s")),
    #"Converted to Table" = Table.FromList(Source, Splitter.SplitByNothing(), null, null, ExtraValues.Error),
    #"Expanded Column1" = Table.ExpandRecordColumn(#"Converted to Table", "Column1", {
        "ops_domain", "account_id", "account_name", "environment", "business_unit", 
        "owner", "cost_center", "service", "resource_or_key", "metric", 
        "actual_value", "expected_value", "potential_savings", "severity", 
        "action_recommended", "timestamp"
    }, {
        "ops_domain", "account_id", "account_name", "environment", "business_unit", 
        "owner", "cost_center", "service", "resource_or_key", "metric", 
        "actual_value", "expected_value", "potential_savings", "severity", 
        "action_recommended", "timestamp"
    })
in
    #"Expanded Column1"
`, endpointURL)
}

// WriteMScriptToFile writes the generated M-code to a .m file.
func WriteMScriptToFile(mCode, outputPath string) error {
	dir := filepath.Dir(outputPath)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return err
		}
	}
	return os.WriteFile(outputPath, []byte(mCode), 0644)
}
