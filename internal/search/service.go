package search

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/moov-io/ach"
	"github.com/moov-io/ach/cmd/achcli/describe/mask"
	"github.com/moov-io/base/log"
	"github.com/moov-io/base/telemetry"
	railmsgsql "github.com/moov-io/rail-msg-sql"
	"github.com/moov-io/rail-msg-sql/internal/achhelp"
	"github.com/moov-io/rail-msg-sql/internal/storage"

	_ "github.com/ncruces/go-sqlite3/driver"
	_ "github.com/ncruces/go-sqlite3/embed"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

type Service interface {
	Close() error

	IngestACHFile(ctx context.Context, filename string, file *ach.File) error
	IngestACHFiles(ctx context.Context, params storage.FilterParams) error

	Search(ctx context.Context, query string, params storage.FilterParams) (*Results, error)
}

func NewService(logger log.Logger, config Config, fileStorage *storage.Repository) (Service, error) {
	db, err := openSqliteDB(config)
	if err != nil {
		return nil, fmt.Errorf("opening %s (as sqlite) failed: %w", config.SqliteFilepath, err)
	}

	// Run migrations
	query, err := railmsgsql.SqliteMigrations.ReadFile("migrations/table_setup.sql")
	if err != nil {
		return nil, fmt.Errorf("opening migrations failed: %w", err)
	}
	_, err = db.Exec(string(query))
	if err != nil {
		return nil, fmt.Errorf("problem running migrations: %w", err)
	}

	return &service{
		logger:      logger,
		config:      config,
		fileStorage: fileStorage,
		db:          db,
	}, nil
}

func openSqliteDB(config Config) (*sql.DB, error) {
	return sql.Open("sqlite3", fmt.Sprintf("file:%s?_foreign_keys=on", config.SqliteFilepath))
}

type service struct {
	logger log.Logger
	config Config

	fileStorage *storage.Repository

	db *sql.DB
}

func (s *service) Close() error {
	if s != nil && s.db != nil {
		return s.db.Close()
	}
	return nil
}

func (s *service) IngestACHFiles(ctx context.Context, params storage.FilterParams) error {
	ctx, span := telemetry.StartSpan(ctx, "ingest-ach-files", trace.WithAttributes(
		attribute.String("ingest.pattern", params.Pattern),
	))
	defer span.End()

	fmt.Printf("service.IngestACHFiles: %#v\n", params)

	listings, err := s.fileStorage.ListAchFiles(ctx, params)
	if err != nil {
		return fmt.Errorf("ingesting ach files: %w", err)
	}
	span.SetAttributes(attribute.Int("ingest.file_count", len(listings)))

	var wg sync.WaitGroup
	wg.Add(len(listings))

	for idx := range listings {
		// Grab each file to ingest
		go func(listing storage.FileListing) {
			defer wg.Done()

			logger := s.logger.With(log.Fields{
				"source_id":    log.String(listing.SourceID),
				"storage_path": log.String(listing.StoragePath),
				"filename":     log.String(listing.Name),
			})

			// Grab the file and store it
			file, err := s.fileStorage.GetAchFile(ctx, listing)
			if err != nil {
				logger.Error().LogErrorf("getting ach file: %v", err)
				return
			}
			if file == nil || file.Contents == nil {
				return // skip files we can't read
			}

			// Store the file
			err = s.IngestACHFile(ctx, file.Filename, file.Contents)
			if err != nil {
				logger.Error().LogErrorf("ingesting file: %v", err)
				return
			}
		}(listings[idx])
	}
	wg.Wait()

	return nil
}

// insertFile inserts an ACH file's header and control into ach_files.
func (s *service) insertFile(ctx context.Context, tx *sql.Tx, filename string, file *ach.File) error {
	ctx, span := telemetry.StartSpan(ctx, "sql-insert-file", trace.WithAttributes(
		attribute.String("filename", filename),
	))
	defer span.End()

	query := `
        INSERT OR IGNORE INTO ach_files (
            file_id,
            filename,
            immediate_destination,
            immediate_origin,
            file_creation_date,
            file_creation_time,
            file_id_modifier,
            immediate_destination_name,
            immediate_origin_name,
            reference_code,
            batch_count,
            block_count,
            entry_addenda_count,
            entry_hash,
            total_debit_entry_dollar_amount,
            total_credit_entry_dollar_amount
        ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
    `
	_, err := tx.ExecContext(ctx, query,
		file.ID,
		filename,
		file.Header.ImmediateDestination,
		file.Header.ImmediateOrigin,
		file.Header.FileCreationDate,
		file.Header.FileCreationTime,
		file.Header.FileIDModifier,
		file.Header.ImmediateDestinationName,
		file.Header.ImmediateOriginName,
		file.Header.ReferenceCode,
		file.Control.BatchCount,
		file.Control.BlockCount,
		file.Control.EntryAddendaCount,
		file.Control.EntryHash,
		file.Control.TotalDebitEntryDollarAmountInFile,
		file.Control.TotalCreditEntryDollarAmountInFile,
	)
	if err != nil {
		return fmt.Errorf("failed to insert ach_file: %w", err)
	}
	return nil
}

// insertBatch inserts a batch's header and control into ach_batches.
func (s *service) insertBatch(ctx context.Context, tx *sql.Tx, fileID string, batch ach.Batcher) error {
	header := batch.GetHeader()
	control := batch.GetControl()
	query := `
        INSERT OR IGNORE INTO ach_batches (
            batch_id,
            file_id,
            service_class_code,
            company_name,
            company_identification,
            standard_entry_class_code,
            company_entry_description,
            company_descriptive_date,
            effective_entry_date,
            settlement_date,
            originator_status_code,
            odfi_identification,
            batch_number,
            service_class_code_control,
            entry_addenda_count_control,
            entry_hash_control,
            total_debit_entry_dollar_amount_control,
            total_credit_entry_dollar_amount_control,
            company_identification_control,
            odfi_identification_control,
            batch_number_control
        ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
    `
	_, err := tx.ExecContext(ctx, query,
		batch.ID(),
		fileID,
		header.ServiceClassCode,
		header.CompanyName,
		header.CompanyIdentification,
		header.StandardEntryClassCode,
		header.CompanyEntryDescription,
		header.CompanyDescriptiveDate,
		header.EffectiveEntryDate,
		header.SettlementDate,
		header.OriginatorStatusCode,
		header.ODFIIdentification,
		header.BatchNumber,
		control.ServiceClassCode,
		control.EntryAddendaCount,
		control.EntryHash,
		control.TotalDebitEntryDollarAmount,
		control.TotalCreditEntryDollarAmount,
		control.CompanyIdentification,
		control.ODFIIdentification,
		control.BatchNumber,
	)
	if err != nil {
		return fmt.Errorf("failed to insert batch: %w", err)
	}
	return nil
}

// insertEntry inserts an entry into ach_entries.
func (s *service) insertEntry(ctx context.Context, tx *sql.Tx, batchID, fileID string, entry *ach.EntryDetail) error {
	query := `
        INSERT OR IGNORE INTO ach_entries (
            entry_id,
            batch_id,
            file_id,
            transaction_code,
            rdfi_identification,
            check_digit,
            dfi_account_number,
            amount,
            individual_identification_number,
            individual_name,
            discretionary_data,
            addenda_record_indicator,
            trace_number
        ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
    `
	_, err := tx.ExecContext(ctx, query,
		entry.ID,
		batchID,
		fileID,
		entry.TransactionCode,
		entry.RDFIIdentification,
		entry.CheckDigit,
		entry.DFIAccountNumber,
		entry.Amount,
		entry.IdentificationNumber,
		entry.IndividualName,
		entry.DiscretionaryData,
		entry.AddendaRecordIndicator,
		entry.TraceNumber,
	)
	if err != nil {
		return fmt.Errorf("failed to insert entry: %w", err)
	}
	return nil
}

// insertAddenda inserts an addenda record into ach_addendas.
func (s *service) insertAddenda(ctx context.Context, tx *sql.Tx, entryID, batchID, fileID string, addenda interface{}) error {
	query := `
        INSERT OR IGNORE INTO ach_addendas (
            entry_id,
            batch_id,
            file_id,
            type_code,
            terminal_identification_code,
            terminal_location,
            terminal_city,
            terminal_state,
            payment_related_information,
            change_code,
            original_entry_trace_number,
            original_rdfi_identification,
            corrected_data,
            refused_change_code,
            refused_original_entry_trace_number,
            refused_original_rdfi_identification,
            refused_corrected_data,
            return_code,
            original_trace_number,
            date_of_death,
            original_receiving_dfi_identification,
            contested_return_code,
            original_entry_trace_number_contested,
            date_original_entry_returned,
            original_receiving_dfi_identification_contested,
            original_settlement_date,
            return_trace_number,
            return_settlement_date,
            return_reason_code,
            dishonored_return_trace_number,
            dishonored_return_settlement_date,
            dishonored_return_reason_code,
            contesting_dfi_identification,
            dishonored_return_code,
            original_entry_trace_number_dishonored,
            return_settlement_date_dishonored,
            original_receiving_dfi_identification_dishonored,
            addenda_information,
            trace_number,
            line_number,
            addenda_sequence_number,
            entry_detail_sequence_number
        ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);
    `

	var values []interface{}

	var typeCode string
	var terminalIdentificationCode, terminalLocation, terminalCity, terminalState interface{}
	var paymentRelatedInformation interface{}
	var changeCode, originalEntryTraceNumber, originalRDFIIdentification, correctedData interface{}
	var refusedChangeCode, refusedOriginalEntryTraceNumber, refusedOriginalRDFIIdentification, refusedCorrectedData interface{}
	var returnCode, originalTraceNumber, dateOfDeath, originalReceivingDFIIdentification interface{}
	var contestedReturnCode, originalEntryTraceNumberContested, dateOriginalEntryReturned, originalReceivingDFIIdentificationContested interface{}
	var originalSettlementDate, returnTraceNumber, returnSettlementDate, returnReasonCode interface{}
	var dishonoredReturnTraceNumber, dishonoredReturnSettlementDate, dishonoredReturnReasonCode, contestingDFIIdentification interface{}
	var dishonoredReturnCode, originalEntryTraceNumberDishonored, returnSettlementDateDishonored, originalReceivingDFIIdentificationDishonored interface{}
	var addendaInformation, traceNumber interface{}
	var lineNumber, addendaSequenceNumber interface{}
	var entryDetailSequenceNumber interface{}

	if a, ok := addenda.(*ach.Addenda02); ok {
		typeCode = "02"
		terminalIdentificationCode = a.TerminalIdentificationCode
		terminalLocation = a.TerminalLocation
		terminalCity = a.TerminalCity
		terminalState = a.TerminalState
	} else if a, ok := addenda.(*ach.Addenda05); ok {
		typeCode = "05"
		paymentRelatedInformation = a.PaymentRelatedInformation
	} else if a, ok := addenda.(*ach.Addenda98); ok {
		typeCode = "98"
		changeCode = a.ChangeCode
		originalEntryTraceNumber = a.OriginalTrace
		originalRDFIIdentification = a.OriginalDFI
		correctedData = a.CorrectedData
	} else if a, ok := addenda.(*ach.Addenda98Refused); ok {
		typeCode = "98R"
		refusedChangeCode = a.ChangeCode
		refusedOriginalEntryTraceNumber = a.OriginalTrace
		refusedOriginalRDFIIdentification = a.OriginalDFI
		refusedCorrectedData = a.CorrectedData
	} else if a, ok := addenda.(*ach.Addenda99); ok {
		typeCode = "99"
		returnCode = a.ReturnCode
		originalTraceNumber = a.OriginalTrace
		dateOfDeath = a.DateOfDeath
		originalReceivingDFIIdentification = a.OriginalDFI
	} else if a, ok := addenda.(*ach.Addenda99Contested); ok {
		typeCode = "99C"
		contestedReturnCode = a.ContestedReturnCode
		originalEntryTraceNumberContested = a.OriginalEntryTraceNumber
		dateOriginalEntryReturned = a.DateOriginalEntryReturned
		originalReceivingDFIIdentificationContested = a.OriginalReceivingDFIIdentification
		originalSettlementDate = a.OriginalSettlementDate
		returnTraceNumber = a.ReturnTraceNumber
		returnSettlementDate = a.ReturnSettlementDate
		returnReasonCode = a.ReturnReasonCode
		dishonoredReturnTraceNumber = a.DishonoredReturnTraceNumber
		dishonoredReturnSettlementDate = a.DishonoredReturnSettlementDate
		dishonoredReturnReasonCode = a.DishonoredReturnReasonCode
		traceNumber = a.TraceNumber
	} else if a, ok := addenda.(*ach.Addenda99Dishonored); ok {
		typeCode = "99D"
		dishonoredReturnCode = a.DishonoredReturnReasonCode
		originalEntryTraceNumberDishonored = a.OriginalEntryTraceNumber
		returnSettlementDateDishonored = a.ReturnSettlementDate
		originalReceivingDFIIdentificationDishonored = a.OriginalReceivingDFIIdentification
		returnReasonCode = a.ReturnReasonCode
		addendaInformation = a.AddendaInformation
		traceNumber = a.TraceNumber
		lineNumber = a.LineNumber
	} else {
		return fmt.Errorf("unsupported addenda type: %T", addenda)
	}

	values = append(values,
		entryID,
		batchID,
		fileID,
		typeCode,
		terminalIdentificationCode,
		terminalLocation,
		terminalCity,
		terminalState,
		paymentRelatedInformation,
		changeCode,
		originalEntryTraceNumber,
		originalRDFIIdentification,
		correctedData,
		refusedChangeCode,
		refusedOriginalEntryTraceNumber,
		refusedOriginalRDFIIdentification,
		refusedCorrectedData,
		returnCode,
		originalTraceNumber,
		dateOfDeath,
		originalReceivingDFIIdentification,
		contestedReturnCode,
		originalEntryTraceNumberContested,
		dateOriginalEntryReturned,
		originalReceivingDFIIdentificationContested,
		originalSettlementDate,
		returnTraceNumber,
		returnSettlementDate,
		returnReasonCode,
		dishonoredReturnTraceNumber,
		dishonoredReturnSettlementDate,
		dishonoredReturnReasonCode,
		contestingDFIIdentification,
		dishonoredReturnCode,
		originalEntryTraceNumberDishonored,
		returnSettlementDateDishonored,
		originalReceivingDFIIdentificationDishonored,
		addendaInformation,
		traceNumber,
		lineNumber,
		addendaSequenceNumber,
		entryDetailSequenceNumber,
	)

	_, err := tx.ExecContext(ctx, query, values...)
	if err != nil {
		return fmt.Errorf("failed to insert addenda (type %s): %w", typeCode, err)
	}
	return nil
}

// IngestACHFile ingests an ACH file into the SQLite database.
func (s *service) IngestACHFile(ctx context.Context, filename string, file *ach.File) error {
	if file == nil {
		return errors.New("nil File")
	}

	ctx, span := telemetry.StartSpan(ctx, "ingest-ach-file", trace.WithAttributes(
		attribute.String("filename", filename),
	))
	defer span.End()

	// Make sure to normalize the IDs
	file = achhelp.PopulateIDs(file)

	// Mask the file before storage
	file = mask.File(file, s.config.AchMasking)

	// Begin transaction
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Insert file
	err = s.insertFile(ctx, tx, filename, file)
	if err != nil {
		return err
	}

	// Insert batches
	for _, batch := range file.Batches {
		err := s.insertBatch(ctx, tx, file.ID, batch)
		if err != nil {
			return err
		}

		// Insert entries
		for _, entry := range batch.GetEntries() {
			err := s.insertEntry(ctx, tx, batch.ID(), file.ID, entry)
			if err != nil {
				return err
			}

			// Insert addenda
			if entry.Addenda02 != nil {
				if err := s.insertAddenda(ctx, tx, entry.ID, batch.ID(), file.ID, entry.Addenda02); err != nil {
					return err
				}
			}
			for _, addenda05 := range entry.Addenda05 {
				if err := s.insertAddenda(ctx, tx, entry.ID, batch.ID(), file.ID, addenda05); err != nil {
					return err
				}
			}
			if entry.Addenda98 != nil {
				if err := s.insertAddenda(ctx, tx, entry.ID, batch.ID(), file.ID, entry.Addenda98); err != nil {
					return err
				}
			}
			if entry.Addenda98Refused != nil {
				if err := s.insertAddenda(ctx, tx, entry.ID, batch.ID(), file.ID, entry.Addenda98Refused); err != nil {
					return err
				}
			}
			if entry.Addenda99 != nil {
				if err := s.insertAddenda(ctx, tx, entry.ID, batch.ID(), file.ID, entry.Addenda99); err != nil {
					return err
				}
			}
			if entry.Addenda99Contested != nil {
				if err := s.insertAddenda(ctx, tx, entry.ID, batch.ID(), file.ID, entry.Addenda99Contested); err != nil {
					return err
				}
			}
			if entry.Addenda99Dishonored != nil {
				if err := s.insertAddenda(ctx, tx, entry.ID, batch.ID(), file.ID, entry.Addenda99Dishonored); err != nil {
					return err
				}
			}
		}
	}

	return tx.Commit()
}

// Search executes a user-provided SQL query with parameters.
func (s *service) Search(ctx context.Context, query string, params storage.FilterParams) (*Results, error) {
	if query == "" {
		return nil, fmt.Errorf("query cannot be empty")
	}

	if params.Pattern != "" {
		query = strings.ReplaceAll(query, "WHERE", "WHERE filename LIKE '%"+params.Pattern+"%' AND ")
	}

	fmt.Printf("\n\n%s\n", query)

	ctx, span := telemetry.StartSpan(ctx, "search-files", trace.WithAttributes(
		attribute.String("sql.query", query),
	))
	defer span.End()

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to execute query: %w", err)
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("failed to get columns: %w", err)
	}

	results := &Results{
		Headers: Row{Columns: make([]interface{}, len(columns))},
	}
	for i, col := range columns {
		results.Headers.Columns[i] = col
	}

	for rows.Next() {
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))

		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}

		results.Rows = append(results.Rows, Row{Columns: values})
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows: %w", err)
	}

	return results, nil
}
