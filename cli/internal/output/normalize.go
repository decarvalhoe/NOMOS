package output

import "sort"

func Normalize(report Report) Report {
	normalized := report
	if normalized.SchemaVersion == "" {
		normalized.SchemaVersion = SchemaVersion
	}
	if normalized.ReportType == "" {
		normalized.ReportType = ReportType
	}

	normalized.Verdict.NextActions = sortedStrings(report.Verdict.NextActions)

	normalized.Checks = append([]Check(nil), report.Checks...)
	for index := range normalized.Checks {
		normalized.Checks[index].FindingIDs = sortedStrings(normalized.Checks[index].FindingIDs)
		normalized.Checks[index].EvidenceIDs = sortedStrings(normalized.Checks[index].EvidenceIDs)
	}
	sort.Slice(normalized.Checks, func(i, j int) bool {
		if normalized.Checks[i].ID == normalized.Checks[j].ID {
			return normalized.Checks[i].Name < normalized.Checks[j].Name
		}
		return normalized.Checks[i].ID < normalized.Checks[j].ID
	})

	normalized.Findings = append([]Finding(nil), report.Findings...)
	for index := range normalized.Findings {
		normalized.Findings[index].EvidenceIDs = sortedStrings(normalized.Findings[index].EvidenceIDs)
	}
	sort.Slice(normalized.Findings, func(i, j int) bool {
		if normalized.Findings[i].ID == normalized.Findings[j].ID {
			return normalized.Findings[i].Code < normalized.Findings[j].Code
		}
		return normalized.Findings[i].ID < normalized.Findings[j].ID
	})

	normalized.Evidence = append([]Evidence(nil), report.Evidence...)
	sort.Slice(normalized.Evidence, func(i, j int) bool {
		if normalized.Evidence[i].ID == normalized.Evidence[j].ID {
			return normalized.Evidence[i].Description < normalized.Evidence[j].Description
		}
		return normalized.Evidence[i].ID < normalized.Evidence[j].ID
	})

	normalized.Waivers = append([]Waiver(nil), report.Waivers...)
	sort.Slice(normalized.Waivers, func(i, j int) bool {
		return normalized.Waivers[i].ID < normalized.Waivers[j].ID
	})

	if report.Links != nil {
		links := *report.Links
		links.Attestations = sortedStrings(links.Attestations)
		normalized.Links = &links
	}

	return normalized
}

func sortedStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	copied := append([]string(nil), values...)
	sort.Strings(copied)
	return copied
}
