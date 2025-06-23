package search

type Results struct {
	Headers Row
	Rows    []Row
}

type Row struct {
	Columns []string
}
