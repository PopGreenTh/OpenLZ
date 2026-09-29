package powerquery

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// resolveEscapedAbsPath converts any file path (relative or absolute) to a fully qualified,
// cleaned absolute Windows path with escaped backslashes for Power Query M-code.
func resolveEscapedAbsPath(filePath string) string {
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		absPath = filePath
	}
	return strings.ReplaceAll(filepath.Clean(absPath), `\`, `\\`)
}

// GenerateParquetMScript produces Power Query M-code to load a local Parquet file into Power BI or Excel.
func GenerateParquetMScript(parquetFilePath string) string {
	escapedPath := resolveEscapedAbsPath(parquetFilePath)
	return fmt.Sprintf(`// OpenLZ Enriched Findings Power Query M-Formula for Power BI Desktop & Excel (Parquet Source)
// 1. Open Power BI Desktop or Excel -> Data -> Get Data -> Blank Query
// 2. Open "Advanced Editor" and replace existing code with this snippet:
let
    Source = Parquet.Document(File.Contents("%s")),
    #"Changed Type" = Table.TransformColumnTypes(Source,{
        {"ops_domain", type text},
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
        {"action_recommended", type text},
        {"timestamp", type datetime}
    })
in
    #"Changed Type"
`, escapedPath)
}

// GenerateEnrichedCsvMScript produces Power Query M-code for the 16-column enriched findings CSV dataset.
func GenerateEnrichedCsvMScript(csvFilePath string) string {
	escapedPath := resolveEscapedAbsPath(csvFilePath)
	return fmt.Sprintf(`// OpenLZ Enriched Findings Power Query M-Formula for Excel & Power BI (CSV Source)
// 1. Open Excel or Power BI Desktop -> Data -> Get Data -> Blank Query
// 2. Open "Advanced Editor" and replace existing code with this snippet:
let
    Source = Csv.Document(File.Contents("%s"),[Delimiter=",", Columns=16, Encoding=65001, QuoteStyle=QuoteStyle.Csv]),
    #"Promoted Headers" = Table.PromoteHeaders(Source, [PromoteAllScalars=true]),
    #"Changed Type" = Table.TransformColumnTypes(#"Promoted Headers",{
        {"ops_domain", type text},
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
        {"action_recommended", type text},
        {"timestamp", type datetime}
    })
in
    #"Changed Type"
`, escapedPath)
}

// GenerateRawFinOpsParquetMScript produces Power Query M-code for the 17-column normalized raw FinOps Parquet dataset.
func GenerateRawFinOpsParquetMScript(parquetFilePath string) string {
	escapedPath := resolveEscapedAbsPath(parquetFilePath)
	return fmt.Sprintf(`// OpenLZ Raw FinOps Power Query M-Formula (Parquet Source)
// 1. Open Power BI Desktop or Excel -> Data -> Get Data -> Blank Query
// 2. Open "Advanced Editor" and replace existing code with this snippet:
let
    Source = Parquet.Document(File.Contents("%s")),
    #"Changed Type" = Table.TransformColumnTypes(Source,{
        {"account_id", type text},
        {"account_name", type text},
        {"service", type text},
        {"usage_date", type text},
        {"usage_period", type text},
        {"amount", type number},
        {"daily_amount", type number},
        {"days_in_period", type number},
        {"metric", type text},
        {"frequency", type text},
        {"unit", type text},
        {"currency", type text},
        {"region", type text},
        {"usage_type", type text},
        {"primary_tag", type text},
        {"secondary_tag", type text},
        {"recorded_at", type datetime}
    })
in
    #"Changed Type"
`, escapedPath)
}

// GenerateRawFinOpsCsvMScript produces Power Query M-code for the 17-column normalized raw FinOps CSV dataset.
func GenerateRawFinOpsCsvMScript(csvFilePath string) string {
	escapedPath := resolveEscapedAbsPath(csvFilePath)
	return fmt.Sprintf(`// OpenLZ Raw FinOps Power Query M-Formula for Excel & Power BI (CSV Source)
// 1. Open Excel or Power BI Desktop -> Data -> Get Data -> Blank Query
// 2. Open "Advanced Editor" and replace existing code with this snippet:
let
    Source = Csv.Document(File.Contents("%s"),[Delimiter=",", Columns=17, Encoding=65001, QuoteStyle=QuoteStyle.Csv]),
    #"Promoted Headers" = Table.PromoteHeaders(Source, [PromoteAllScalars=true]),
    #"Changed Type" = Table.TransformColumnTypes(#"Promoted Headers",{
        {"account_id", type text},
        {"account_name", type text},
        {"service", type text},
        {"usage_date", type text},
        {"usage_period", type text},
        {"amount", type number},
        {"daily_amount", type number},
        {"days_in_period", type number},
        {"metric", type text},
        {"frequency", type text},
        {"unit", type text},
        {"currency", type text},
        {"region", type text},
        {"usage_type", type text},
        {"primary_tag", type text},
        {"secondary_tag", type text},
        {"recorded_at", type datetime}
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
