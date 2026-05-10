package dataset

import (
	"bufio"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/gocarina/gocsv"
)

type textLineHandler func(line string) (accepted bool, err error)

func scanTextSource(
	path string,
	skipLines int,
	handler textLineHandler,
) (acceptedCount int, err error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, fmt.Errorf("open text source %q: %w", path, err)
	}
	defer func() { _ = f.Close() }()

	br, err := newBOMStrippedReader(f)
	if err != nil {
		return 0, fmt.Errorf("prepare text source %q: %w", path, err)
	}

	for i := range skipLines {
		if _, err := br.ReadString('\n'); err != nil {
			return 0, fmt.Errorf("skip header line %d in %q: %w", i+1, path, err)
		}
	}

	return scanTextBody(br, path, handler)
}

func scanTextBody(
	r io.Reader,
	path string,
	handler textLineHandler,
) (acceptedCount int, err error) {
	scanner := bufio.NewScanner(r)
	lineNumber := 0
	for scanner.Scan() {
		lineNumber++
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			return 0, fmt.Errorf("handle text source %q at line %d: unexpected empty line", path, lineNumber)
		}

		accepted, err := handler(line)
		if err != nil {
			return 0, fmt.Errorf("handle text source %q at line %d: %w", path, lineNumber, err)
		}

		if accepted {
			acceptedCount++
		}
	}

	if err = scanner.Err(); err != nil {
		return 0, fmt.Errorf("scan text source %q: %w", path, err)
	}

	return acceptedCount, nil
}

type csvRecordHandler[T any] func(record T) (accepted bool, err error)

type csvReaderConfigurer func(r *csv.Reader)

func scanCSVSource[T any](
	path string,
	skipLines int,
	configure csvReaderConfigurer,
	handler csvRecordHandler[T],
) (acceptedCount int, err error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, fmt.Errorf("open CSV source %q: %w", path, err)
	}
	defer func() { _ = f.Close() }()

	br, err := newBOMStrippedReader(f)
	if err != nil {
		return 0, fmt.Errorf("prepare CSV source %q: %w", path, err)
	}

	for i := 0; i < skipLines; i++ {
		if _, err := br.ReadString('\n'); err != nil {
			return 0, fmt.Errorf("skip header line %d in %q: %w", i+1, path, err)
		}
	}

	return scanCSVBody(br, path, configure, handler)
}

func scanCSVBody[T any](
	r io.Reader,
	path string,
	configure csvReaderConfigurer,
	handler csvRecordHandler[T],
) (acceptedCount int, err error) {
	rdr := csv.NewReader(r)
	if configure != nil {
		configure(rdr)
	}

	var zero T
	um, err := gocsv.NewUnmarshaller(rdr, zero)
	if err != nil {
		return 0, fmt.Errorf("unmarshal CSV source %q: %w", path, err)
	}

	for i := 1; ; i++ {
		recordAny, err := um.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return 0, fmt.Errorf("read CSV source %q at record %d: %w", path, i, err)
		}

		record, ok := recordAny.(T)
		if !ok {
			return 0, fmt.Errorf(
				"read CSV source %q at record %d: unexpected record type %T",
				path,
				i,
				recordAny,
			)
		}

		accepted, handleErr := handler(record)
		if handleErr != nil {
			return 0, fmt.Errorf("handle CSV source %q at record %d: %w", path, i, handleErr)
		}

		if accepted {
			acceptedCount++
		}
	}

	return acceptedCount, nil
}

type jsonArrayHandler[T any] func(record T) error

func decodeJSONArray[T any](r io.Reader, handle jsonArrayHandler[T]) (int, error) {
	br, err := newBOMStrippedReader(r)
	if err != nil {
		return 0, fmt.Errorf("prepare JSON source: %w", err)
	}

	dec := json.NewDecoder(br)
	tok, err := dec.Token()
	if err != nil {
		return 0, fmt.Errorf("read start token: %w", err)
	}
	if delim, ok := tok.(json.Delim); !ok || delim != '[' {
		return 0, fmt.Errorf("must be an array")
	}

	count := 0
	for dec.More() {
		var elem T
		if err := dec.Decode(&elem); err != nil {
			return 0, fmt.Errorf("decode entry: %w", err)
		}

		if err := handle(elem); err != nil {
			return 0, err
		}

		count++
	}

	if tok, err = dec.Token(); err != nil {
		return 0, fmt.Errorf("read end token: %w", err)
	}
	if delim, ok := tok.(json.Delim); !ok || delim != ']' {
		return 0, fmt.Errorf("has invalid array termination")
	}
	if tok, err = dec.Token(); err != io.EOF {
		if err != nil {
			return 0, fmt.Errorf("read trailing data: %w", err)
		}

		return 0, fmt.Errorf("contains trailing token %v", tok)
	}

	return count, nil
}

func decodeJSON[T any](r io.Reader, dst *T) error {
	br, err := newBOMStrippedReader(r)
	if err != nil {
		return fmt.Errorf("prepare JSON source: %w", err)
	}

	dec := json.NewDecoder(br)
	if err := dec.Decode(dst); err != nil {
		return fmt.Errorf("decode value: %w", err)
	}

	if tok, err := dec.Token(); err != io.EOF {
		if err != nil {
			return fmt.Errorf("read trailing data: %w", err)
		}

		return fmt.Errorf("contains trailing token %v", tok)
	}

	return nil
}

func newBOMStrippedReader(r io.Reader) (*bufio.Reader, error) {
	br := bufio.NewReader(r)

	b, err := br.Peek(3)
	if err != nil && err != io.EOF {
		return nil, fmt.Errorf("peek UTF-8 BOM: %w", err)
	}

	if len(b) >= 3 && b[0] == 0xEF && b[1] == 0xBB && b[2] == 0xBF {
		_, _ = br.Discard(3)
	}

	return br, nil
}
