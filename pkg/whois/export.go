package whois

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"sync"

	whoisparser "github.com/likexian/whois-parser"
)

// ExportToJSON 将WHOIS信息导出为JSON
func ExportToJSON(info *whoisparser.WhoisInfo, w io.Writer) error {
	if info == nil {
		return fmt.Errorf("WHOIS信息不能为空")
	}
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(info)
}

// ExportToCSV 将WHOIS信息导出为CSV
func ExportToCSV(info *whoisparser.WhoisInfo, w io.Writer) error {
	if info == nil {
		return fmt.Errorf("WHOIS信息不能为空")
	}

	writer := csv.NewWriter(w)
	defer writer.Flush()

	// 写入表头
	if err := writer.Write([]string{"Field", "Value"}); err != nil {
		return err
	}

	// 域名字段
	if info.Domain != nil {
		rows := [][2]string{
			{"Domain", info.Domain.Domain},
			{"CreatedDate", info.Domain.CreatedDate},
			{"UpdatedDate", info.Domain.UpdatedDate},
			{"ExpirationDate", info.Domain.ExpirationDate},
			{"WhoisServer", info.Domain.WhoisServer},
			{"Status", strings.Join(info.Domain.Status, ", ")},
			{"NameServers", strings.Join(info.Domain.NameServers, ", ")},
			{"DNSSec", fmt.Sprintf("%v", info.Domain.DNSSec)},
		}
		for _, row := range rows {
			if err := writer.Write([]string{row[0], row[1]}); err != nil {
				return err
			}
		}
	}

	// 联系人字段
	contacts := []struct {
		name    string
		contact *whoisparser.Contact
	}{
		{"Registrar", info.Registrar},
		{"Registrant", info.Registrant},
		{"Administrative", info.Administrative},
		{"Technical", info.Technical},
		{"Billing", info.Billing},
	}
	for _, c := range contacts {
		if c.contact != nil {
			rows := [][2]string{
				{fmt.Sprintf("%s.Name", c.name), c.contact.Name},
				{fmt.Sprintf("%s.Organization", c.name), c.contact.Organization},
				{fmt.Sprintf("%s.Email", c.name), c.contact.Email},
				{fmt.Sprintf("%s.Phone", c.name), c.contact.Phone},
				{fmt.Sprintf("%s.Country", c.name), c.contact.Country},
				{fmt.Sprintf("%s.City", c.name), c.contact.City},
				{fmt.Sprintf("%s.Street", c.name), c.contact.Street},
			}
			for _, row := range rows {
				if err := writer.Write([]string{row[0], row[1]}); err != nil {
					return err
				}
			}
		}
	}

	return nil
}

// ExportToMarkdown 将WHOIS信息导出为Markdown表格
func ExportToMarkdown(info *whoisparser.WhoisInfo, w io.Writer) error {
	if info == nil {
		return fmt.Errorf("WHOIS信息不能为空")
	}

	var sb strings.Builder

	sb.WriteString("# WHOIS 查询结果\n\n")

	if info.Domain != nil {
		sb.WriteString("## 域名信息\n\n")
		sb.WriteString("| 字段 | 值 |\n")
		sb.WriteString("|------|----|\n")
		sb.WriteString(fmt.Sprintf("| 域名 | %s |\n", info.Domain.Domain))
		sb.WriteString(fmt.Sprintf("| 创建日期 | %s |\n", info.Domain.CreatedDate))
		sb.WriteString(fmt.Sprintf("| 更新日期 | %s |\n", info.Domain.UpdatedDate))
		sb.WriteString(fmt.Sprintf("| 过期日期 | %s |\n", info.Domain.ExpirationDate))
		sb.WriteString(fmt.Sprintf("| WHOIS服务器 | %s |\n", info.Domain.WhoisServer))
		sb.WriteString(fmt.Sprintf("| 状态 | %s |\n", strings.Join(info.Domain.Status, ", ")))
		sb.WriteString(fmt.Sprintf("| 域名服务器 | %s |\n", strings.Join(info.Domain.NameServers, ", ")))
		sb.WriteString(fmt.Sprintf("| DNSSec | %v |\n", info.Domain.DNSSec))
		sb.WriteString("\n")
	}

	// 联系人信息
	contacts := []struct {
		name    string
		contact *whoisparser.Contact
	}{
		{"注册商", info.Registrar},
		{"注册人", info.Registrant},
		{"管理联系人", info.Administrative},
		{"技术联系人", info.Technical},
		{"账单联系人", info.Billing},
	}

	for _, c := range contacts {
		if c.contact != nil {
			sb.WriteString(fmt.Sprintf("## %s\n\n", c.name))
			sb.WriteString("| 字段 | 值 |\n")
			sb.WriteString("|------|----|\n")
			sb.WriteString(fmt.Sprintf("| 名称 | %s |\n", c.contact.Name))
			sb.WriteString(fmt.Sprintf("| 组织 | %s |\n", c.contact.Organization))
			sb.WriteString(fmt.Sprintf("| 邮箱 | %s |\n", c.contact.Email))
			sb.WriteString(fmt.Sprintf("| 电话 | %s |\n", c.contact.Phone))
			sb.WriteString(fmt.Sprintf("| 国家 | %s |\n", c.contact.Country))
			sb.WriteString(fmt.Sprintf("| 城市 | %s |\n", c.contact.City))
			sb.WriteString(fmt.Sprintf("| 地址 | %s |\n", c.contact.Street))
			sb.WriteString("\n")
		}
	}

	_, err := io.WriteString(w, sb.String())
	return err
}

// ============================================================================
// 导出格式扩展点
//
// Exporter 接口抽象导出能力，内置 json/csv/markdown 三种注册式实现。
// 上层可通过 RegisterExporter 注入自定义格式（如 stix/cef/自定义 JSON schema），
// 通过 ExportWith 按 format 名分发。
// ============================================================================

// Exporter WHOIS 信息导出器接口。
type Exporter interface {
	// Format 返回格式名（如 json/csv/markdown）。
	Format() string
	// Export 将 WHOIS 信息导出到 writer。
	Export(info *whoisparser.WhoisInfo, w io.Writer) error
}

// exporterRegistry 导出器注册表。
var exporterRegistry = struct {
	mu        sync.RWMutex
	exporters map[string]Exporter
}{
	exporters: make(map[string]Exporter),
}

func init() {
	// 注册内置导出器
	RegisterExporter(&jsonExporter{})
	RegisterExporter(&csvExporter{})
	RegisterExporter(&markdownExporter{})
}

// RegisterExporter 注册导出器（线程安全）。同名覆盖。
func RegisterExporter(e Exporter) {
	if e == nil {
		return
	}
	exporterRegistry.mu.Lock()
	defer exporterRegistry.mu.Unlock()
	exporterRegistry.exporters[e.Format()] = e
}

// GetExporter 按格式名获取导出器。
func GetExporter(format string) (Exporter, bool) {
	exporterRegistry.mu.RLock()
	defer exporterRegistry.mu.RUnlock()
	e, ok := exporterRegistry.exporters[format]
	return e, ok
}

// ListExporters 列出已注册的所有导出器格式名。
func ListExporters() []string {
	exporterRegistry.mu.RLock()
	defer exporterRegistry.mu.RUnlock()
	formats := make([]string, 0, len(exporterRegistry.exporters))
	for f := range exporterRegistry.exporters {
		formats = append(formats, f)
	}
	return formats
}

// ExportWith 按格式名导出（走注册表分发）。
func ExportWith(info *whoisparser.WhoisInfo, format string, w io.Writer) error {
	e, ok := GetExporter(format)
	if !ok {
		return fmt.Errorf("未注册的导出格式: %s（已注册: %v）", format, ListExporters())
	}
	return e.Export(info, w)
}

// UnregisterExporter 注销导出器（主要用于测试）。
func UnregisterExporter(format string) {
	exporterRegistry.mu.Lock()
	defer exporterRegistry.mu.Unlock()
	delete(exporterRegistry.exporters, format)
}

// ---- 内置导出器实现 ----

// jsonExporter JSON 导出器。
type jsonExporter struct{}

func (e *jsonExporter) Format() string { return "json" }
func (e *jsonExporter) Export(info *whoisparser.WhoisInfo, w io.Writer) error {
	return ExportToJSON(info, w)
}

// csvExporter CSV 导出器。
type csvExporter struct{}

func (e *csvExporter) Format() string { return "csv" }
func (e *csvExporter) Export(info *whoisparser.WhoisInfo, w io.Writer) error {
	return ExportToCSV(info, w)
}

// markdownExporter Markdown 导出器。
type markdownExporter struct{}

func (e *markdownExporter) Format() string { return "markdown" }
func (e *markdownExporter) Export(info *whoisparser.WhoisInfo, w io.Writer) error {
	return ExportToMarkdown(info, w)
}
