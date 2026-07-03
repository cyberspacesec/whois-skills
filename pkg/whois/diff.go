package whois

import (
	"fmt"
	"reflect"
	"strings"

	whoisparser "github.com/likexian/whois-parser"
)

// ChangeType 变更类型
type ChangeType string

const (
	ChangeAdded    ChangeType = "added"
	ChangeRemoved  ChangeType = "removed"
	ChangeModified ChangeType = "modified"
)

// WhoisChange WHOIS变更记录
type WhoisChange struct {
	// 变更类型
	Type ChangeType `json:"type"`

	// 变更字段
	Field string `json:"field"`

	// 旧值
	OldValue interface{} `json:"old_value,omitempty"`

	// 新值
	NewValue interface{} `json:"new_value,omitempty"`

	// 字段路径
	Path string `json:"path"`
}

// CompareWhois 比较两份WHOIS信息的差异
func CompareWhois(old, new *whoisparser.WhoisInfo) []*WhoisChange {
	var changes []*WhoisChange

	if old == nil && new == nil {
		return changes
	}
	if old == nil {
		return []*WhoisChange{{Type: ChangeAdded, Field: "whois_info", NewValue: new, Path: "."}}
	}
	if new == nil {
		return []*WhoisChange{{Type: ChangeRemoved, Field: "whois_info", OldValue: old, Path: "."}}
	}

	// 比较域名信息
	changes = append(changes, compareDomain(old.Domain, new.Domain)...)

	// 比较联系人信息
	changes = append(changes, compareContact("registrar", old.Registrar, new.Registrar)...)
	changes = append(changes, compareContact("registrant", old.Registrant, new.Registrant)...)
	changes = append(changes, compareContact("administrative", old.Administrative, new.Administrative)...)
	changes = append(changes, compareContact("technical", old.Technical, new.Technical)...)
	changes = append(changes, compareContact("billing", old.Billing, new.Billing)...)

	return changes
}

// compareDomain 比较域名信息
func compareDomain(old, new *whoisparser.Domain) []*WhoisChange {
	var changes []*WhoisChange
	if old == nil && new == nil {
		return changes
	}
	if old == nil {
		return []*WhoisChange{{Type: ChangeAdded, Field: "domain", NewValue: new, Path: "domain"}}
	}
	if new == nil {
		return []*WhoisChange{{Type: ChangeRemoved, Field: "domain", OldValue: old, Path: "domain"}}
	}

	// 比较字符串字段
	stringFields := []struct {
		name string
		old  string
		new  string
	}{
		{"domain.created_date", old.CreatedDate, new.CreatedDate},
		{"domain.updated_date", old.UpdatedDate, new.UpdatedDate},
		{"domain.expiration_date", old.ExpirationDate, new.ExpirationDate},
		{"domain.whois_server", old.WhoisServer, new.WhoisServer},
		{"domain.dnssec", fmt.Sprintf("%v", old.DNSSec), fmt.Sprintf("%v", new.DNSSec)},
	}
	for _, f := range stringFields {
		if f.old != f.new {
			changes = append(changes, &WhoisChange{
				Type:     ChangeModified,
				Field:    f.name,
				OldValue: f.old,
				NewValue: f.new,
				Path:     f.name,
			})
		}
	}

	// 比较切片字段 (Status, NameServers)
	changes = append(changes, compareStringSlices("domain.status", old.Status, new.Status)...)
	changes = append(changes, compareStringSlices("domain.name_servers", old.NameServers, new.NameServers)...)

	return changes
}

// compareContact 比较联系人信息
func compareContact(section string, old, new *whoisparser.Contact) []*WhoisChange {
	var changes []*WhoisChange
	if old == nil && new == nil {
		return changes
	}
	if old == nil && new != nil {
		return []*WhoisChange{{Type: ChangeAdded, Field: section, NewValue: new, Path: section}}
	}
	if old != nil && new == nil {
		return []*WhoisChange{{Type: ChangeRemoved, Field: section, OldValue: old, Path: section}}
	}

	// 使用反射比较联系人字段
	vOld := reflect.ValueOf(old).Elem()
	vNew := reflect.ValueOf(new).Elem()
	tOld := vOld.Type()

	for i := 0; i < vOld.NumField(); i++ {
		fieldName := tOld.Field(i).Name
		oldVal := vOld.Field(i).String()
		newVal := vNew.Field(i).String()
		if oldVal != newVal {
			path := fmt.Sprintf("%s.%s", section, strings.ToLower(fieldName))
			changes = append(changes, &WhoisChange{
				Type:     ChangeModified,
				Field:    path,
				OldValue: oldVal,
				NewValue: newVal,
				Path:     path,
			})
		}
	}
	return changes
}

// compareStringSlices 比较字符串切片
func compareStringSlices(path string, old, new []string) []*WhoisChange {
	var changes []*WhoisChange
	oldSet := make(map[string]bool)
	for _, s := range old {
		oldSet[strings.ToLower(s)] = true
	}
	newSet := make(map[string]bool)
	for _, s := range new {
		newSet[strings.ToLower(s)] = true
	}
	for s := range oldSet {
		if !newSet[s] {
			changes = append(changes, &WhoisChange{Type: ChangeRemoved, Field: path, OldValue: s, Path: path})
		}
	}
	for s := range newSet {
		if !oldSet[s] {
			changes = append(changes, &WhoisChange{Type: ChangeAdded, Field: path, NewValue: s, Path: path})
		}
	}
	return changes
}
