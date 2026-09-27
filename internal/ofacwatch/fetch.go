package ofacwatch

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
)

const (
	sdnURL             = "https://sanctionslistservice.ofac.treas.gov/api/download/SDN.CSV"
	ecfrTitle31URL     = "https://www.ecfr.gov/api/versioner/v1/structure/current/title-31.json"
	federalRegisterURL = "https://www.federalregister.gov/api/v1/documents.json"

	sdnProgramColumn   = 3
	ofacFirstCFRPart   = 500
	ofacLastCFRPart    = 599
	federalRegisterMax = "20"
)

// Region changes in Ukraine happen under these existing orders, without a new program or part.
var federalRegisterTerms = []string{`"Executive Order 13685"`, `"Executive Order 14065"`}

var (
	errNoSDNPrograms = errors.New("no SDN programs found")
	errNoCFRParts    = errors.New("no OFAC parts found in 31 CFR")
)

func (w *Watcher) fetch(ctx context.Context) (snapshot, error) {
	var s snapshot
	var err error

	if s.SDNPrograms, err = fetchParsed(ctx, w.client, sdnURL, parseSDNPrograms); err != nil {
		return snapshot{}, fmt.Errorf("SDN list: %w", err)
	}
	if s.CFRParts, err = fetchParsed(ctx, w.client, ecfrTitle31URL, parseCFRParts); err != nil {
		return snapshot{}, fmt.Errorf("eCFR: %w", err)
	}
	seen := make(map[string]struct{})
	for _, term := range federalRegisterTerms {
		documents, err := fetchParsed(ctx, w.client, federalRegisterQuery(term), parseDocuments)
		if err != nil {
			return snapshot{}, fmt.Errorf("federal register: %w", err)
		}
		for _, d := range documents {
			if _, ok := seen[d.Number]; !ok {
				seen[d.Number] = struct{}{}
				s.Documents = append(s.Documents, d)
			}
		}
	}
	return s, nil
}

func fetchParsed[T any](
	ctx context.Context,
	client *http.Client,
	rawURL string,
	parse func(io.Reader) (T, error),
) (T, error) {
	var zero T
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return zero, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return zero, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return zero, fmt.Errorf("unexpected status %d", resp.StatusCode)
	}
	return parse(resp.Body)
}

func federalRegisterQuery(term string) string {
	query := url.Values{
		"conditions[term]": {term},
		"order":            {"newest"},
		"per_page":         {federalRegisterMax},
		"fields[]":         {"document_number", "title", "html_url"},
	}
	return federalRegisterURL + "?" + query.Encode()
}

// parseSDNPrograms reads the program column, where multiple programs look like "SDGT] [IRGC".
func parseSDNPrograms(r io.Reader) ([]string, error) {
	reader := csv.NewReader(r)
	reader.FieldsPerRecord = -1
	reader.LazyQuotes = true

	seen := make(map[string]struct{})
	for {
		record, err := reader.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
		if len(record) <= sdnProgramColumn {
			continue
		}
		for program := range strings.SplitSeq(record[sdnProgramColumn], "] [") {
			if program = strings.Trim(program, "[] "); program != "" {
				seen[program] = struct{}{}
			}
		}
	}
	if len(seen) == 0 {
		return nil, errNoSDNPrograms
	}

	programs := make([]string, 0, len(seen))
	for program := range seen {
		programs = append(programs, program)
	}
	slices.Sort(programs)
	return programs, nil
}

type ecfrNode struct {
	Type        string     `json:"type"`
	Identifier  string     `json:"identifier"`
	Description string     `json:"label_description"`
	Reserved    bool       `json:"reserved"`
	Children    []ecfrNode `json:"children"`
}

// parseCFRParts returns active parts of 31 CFR chapter V, which holds the OFAC regulations.
func parseCFRParts(r io.Reader) ([]cfrPart, error) {
	var root ecfrNode
	if err := json.NewDecoder(r).Decode(&root); err != nil {
		return nil, err
	}

	var parts []cfrPart
	var walk func(ecfrNode)
	walk = func(n ecfrNode) {
		if n.Type == "part" && !n.Reserved && isOFACPart(n.Identifier) {
			parts = append(parts, cfrPart{ID: n.Identifier, Title: n.Description})
		}
		for _, child := range n.Children {
			walk(child)
		}
	}
	walk(root)

	if len(parts) == 0 {
		return nil, errNoCFRParts
	}
	return parts, nil
}

func isOFACPart(identifier string) bool {
	part, err := strconv.Atoi(identifier)
	return err == nil && part >= ofacFirstCFRPart && part <= ofacLastCFRPart
}

func parseDocuments(r io.Reader) ([]document, error) {
	var resp struct {
		Results []struct {
			Number string `json:"document_number"`
			Title  string `json:"title"`
			URL    string `json:"html_url"`
		} `json:"results"`
	}
	if err := json.NewDecoder(r).Decode(&resp); err != nil {
		return nil, err
	}

	documents := make([]document, 0, len(resp.Results))
	for _, result := range resp.Results {
		documents = append(documents, document{Number: result.Number, Title: result.Title, URL: result.URL})
	}
	return documents, nil
}
