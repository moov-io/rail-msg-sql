CREATE TABLE ach_files (
    file_id TEXT PRIMARY KEY NOT NULL,
    filename TEXT NOT NULL,
    immediate_destination TEXT,
    immediate_origin TEXT,
    file_creation_date TEXT,
    file_creation_time TEXT,
    file_id_modifier TEXT,
    immediate_destination_name TEXT,
    immediate_origin_name TEXT,
    reference_code TEXT,
    batch_count INTEGER,
    block_count INTEGER,
    entry_addenda_count INTEGER,
    entry_hash INTEGER,
    total_debit_entry_dollar_amount INTEGER,
    total_credit_entry_dollar_amount INTEGER,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE UNIQUE INDEX ach_files_uniq_idx ON ach_files(file_id, filename);

CREATE TABLE ach_batches (
    batch_id TEXT NOT NULL,
    file_id INTEGER NOT NULL,
    service_class_code INTEGER,
    company_name TEXT,
    company_identification TEXT,
    standard_entry_class_code TEXT,
    company_entry_description TEXT,
    company_descriptive_date TEXT,
    effective_entry_date TEXT,
    settlement_date TEXT,
    originator_status_code TEXT,
    odfi_identification TEXT,
    batch_number INTEGER,
    service_class_code_control INTEGER,
    entry_addenda_count_control INTEGER,
    entry_hash_control INTEGER,
    total_debit_entry_dollar_amount_control INTEGER,
    total_credit_entry_dollar_amount_control INTEGER,
    company_identification_control TEXT,
    odfi_identification_control TEXT,
    batch_number_control INTEGER,
    FOREIGN KEY (file_id) REFERENCES ach_files(file_id) ON DELETE CASCADE
);

CREATE UNIQUE INDEX ach_batches_uniq_idx ON ach_batches(batch_id, file_id);

CREATE TABLE ach_entries (
    entry_id TEXT NOT NULL,
    batch_id INTEGER NOT NULL,
    file_id TEXT NOT NULL,
    transaction_code INTEGER,
    rdfi_identification TEXT,
    check_digit TEXT,
    dfi_account_number TEXT,
    amount INTEGER,
    individual_identification_number TEXT,
    individual_name TEXT,
    discretionary_data TEXT,
    addenda_record_indicator INTEGER,
    trace_number TEXT,
    FOREIGN KEY (file_id) REFERENCES ach_files(file_id) ON DELETE CASCADE
);

CREATE UNIQUE INDEX ach_entries_uniq_idx ON ach_entries(entry_id, batch_id, file_id);

CREATE TABLE ach_addendas (
    entry_id INTEGER UNIQUE NOT NULL,
    batch_id INTEGER NOT NULL,
    file_id TEXT NOT NULL,
    type_code TEXT NOT NULL,
    terminal_identification_code TEXT,
    terminal_location TEXT,
    terminal_city TEXT,
    terminal_state TEXT,
    payment_related_information TEXT,
    change_code TEXT,
    original_entry_trace_number TEXT,
    original_rdfi_identification TEXT,
    corrected_data TEXT,
    refused_change_code TEXT,
    refused_original_entry_trace_number TEXT,
    refused_original_rdfi_identification TEXT,
    refused_corrected_data TEXT,
    return_code TEXT,
    original_trace_number TEXT,
    date_of_death TEXT,
    original_receiving_dfi_identification TEXT,
    contested_return_code TEXT,
    original_entry_trace_number_contested TEXT,
    date_original_entry_returned TEXT,
    original_receiving_dfi_identification_contested TEXT,
    original_settlement_date TEXT,
    return_trace_number TEXT,
    return_settlement_date TEXT,
    return_reason_code TEXT,
    dishonored_return_trace_number TEXT,
    dishonored_return_settlement_date TEXT,
    dishonored_return_reason_code TEXT,
    contesting_dfi_identification TEXT,
    dishonored_return_code TEXT,
    original_entry_trace_number_dishonored TEXT,
    return_settlement_date_dishonored TEXT,
    original_receiving_dfi_identification_dishonored TEXT,
    addenda_information TEXT,
    trace_number TEXT,
    line_number INTEGER,
    addenda_sequence_number INTEGER,
    entry_detail_sequence_number TEXT,
    FOREIGN KEY (file_id) REFERENCES ach_files(file_id) ON DELETE CASCADE
);

CREATE UNIQUE INDEX ach_addendas_uniq_idx ON ach_addendas(entry_id, batch_id, file_id);
