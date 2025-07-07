// Predefined queries data structure
const predefinedQueries = [
  {
    category: "Files",
    queries: [
      { name: "Recent Files", query: `
SELECT
  file_id, filename, file_creation_date as creation_date, file_creation_time as creation_time,
  immediate_destination_name as destination, immediate_origin_name as origin ,
  total_debit_entry_dollar_amount as debits, total_credit_entry_dollar_amount as credits

FROM ach_files
WHERE created_at >= datetime('now', '-7 days')
ORDER BY created_at DESC
LIMIT 5;
`},
      { name: "Largest Amounts in Files", query: `
WITH RankedEntries AS (
    SELECT e.entry_id, e.file_id,
           e.individual_name, e.dfi_account_number, e.amount,
           e.transaction_code, e.trace_number, f.filename, f.file_creation_date,
           (
               SELECT COUNT(*) + 1
               FROM ach_entries e2
               WHERE e2.file_id = e.file_id
               AND e2.amount > e.amount
           ) AS rank_num
    FROM ach_entries e
    JOIN ach_files f ON e.file_id = f.file_id
    WHERE f.created_at >= datetime('now', '-7 days')
)
SELECT
  -- entry_id, file_id,
  individual_name, dfi_account_number, amount, transaction_code,
  trace_number, filename, file_creation_date
FROM RankedEntries
WHERE rank_num <= 3
ORDER BY file_id, amount DESC
LIMIT 10;
`},
    ],
  },
  {
    category: "Batches",
    queries: [
      { name: "Originator Activity Summary", query: `
SELECT
  b.company_name, b.company_identification, COUNT(DISTINCT b.batch_id) AS batch_count,
  SUM(e.amount) AS total_amount, COUNT(e.entry_id) AS entry_count, f.filename

FROM ach_batches b
JOIN ach_entries e ON b.batch_id = e.batch_id AND b.file_id = e.file_id
JOIN ach_files f ON b.file_id = f.file_id

WHERE
  -- b.company_identification = :company_id AND
  f.created_at >= datetime('now', '-30 days')

GROUP BY b.company_name, b.company_identification, f.filename
ORDER BY total_amount DESC
LIMIT 10;
`},
    ],
  },
  {
    category: "Entries",
    queries: [
      { name: "Entries By Name or Account Number", query: `
SELECT
  -- e.entry_id, e.file_id,
  e.transaction_code, e.amount,
  e.dfi_account_number, e.individual_name,
  e.trace_number, f.filename, f.file_creation_date

FROM ach_entries e
JOIN ach_files f ON e.file_id = f.file_id

WHERE
  e.individual_name LIKE '%'
  -- e.dfi_account_number LIKE '%1234'
  AND created_at >= datetime('now', '-7 days')

ORDER BY f.created_at DESC
LIMIT 10;
` },
      { name: "Possible Duplicate Entries", query: `
SELECT
  e.dfi_account_number, e.amount, e.individual_name, e.trace_number, COUNT(*) AS entry_count,
  -- GROUP_CONCAT(e.entry_id) AS entry_ids,
  f.filename

FROM ach_entries e
JOIN ach_files f ON e.file_id = f.file_id

WHERE
  f.created_at >= datetime('now', '-30 days')

GROUP BY e.dfi_account_number, e.amount, e.trace_number
HAVING entry_count > 1
ORDER BY f.created_at DESC
LIMIT 10;

`},
    ],
  },
  {
    category: "Exceptions",
    queries: [
      { name: "Recent Corrections", query: `
SELECT
  -- e.entry_id,
  e.individual_name, e.dfi_account_number, e.amount,
  e.trace_number, a.change_code,
  -- a.payment_related_information,
  f.filename, f.file_creation_date

FROM ach_entries e
JOIN ach_addendas a ON e.entry_id = a.entry_id AND e.file_id = a.file_id
JOIN ach_files f ON e.file_id = f.file_id

WHERE a.change_code IS NOT NULL

ORDER BY f.created_at DESC
LIMIT 10;
`},
      { name: "Recent Returns", query: `
SELECT
  -- e.entry_id,
  e.individual_name, e.dfi_account_number, e.amount,
  e.trace_number, a.return_code,
  -- a.payment_related_information,
  f.filename, f.file_creation_date

FROM ach_entries e
JOIN ach_addendas a ON e.entry_id = a.entry_id AND e.file_id = a.file_id
JOIN ach_files f ON e.file_id = f.file_id

WHERE a.return_code IS NOT NULL

ORDER BY f.created_at DESC
LIMIT 10;
`},
    ],
  },
];

// Function to format date as YYYY-MM-DD
function yyyymmdd(date) {
  if (!date || isNaN(date.getTime())) return "Unknown";
  const year = date.getFullYear();
  const month = String(date.getMonth() + 1).padStart(2, "0");
  const day = String(date.getDate()).padStart(2, "0");
  return `${year}-${month}-${day}`;
}

// Function to validate and parse date from string
function parseDate(dateStr) {
  if (!dateStr || typeof dateStr !== "string") return null;
  const regex = /^\d{4}-\d{2}-\d{2}$/;
  if (!regex.test(dateStr)) return null;

  const date = new Date(dateStr);
  if (isNaN(date.getTime())) return null;

  const minDate = new Date("1970-01-01");
  const maxDate = new Date();
  maxDate.setFullYear(maxDate.getFullYear() + 1);
  if (date < minDate || date > maxDate) return null;

  return date;
}

// Function to calculate start and end dates from query params
function calculateStartEndDate() {
  const params = new URLSearchParams(window.location.search);
  let startDate = parseDate(params.get("startDate"));
  let endDate = parseDate(params.get("endDate"));

  // Only fall back to default if both dates are null or invalid
  if (!startDate || !endDate) {
    endDate = new Date(); // Today, 10:10 AM CDT, July 07, 2025
    endDate.setHours(23, 59, 59, 999);
    startDate = new Date(endDate);
    startDate.setDate(endDate.getDate() - 7);
    startDate.setHours(0, 0, 0, 0);
  } else if (endDate < startDate) {
    console.warn("endDate is before startDate, using default range");
    endDate = new Date();
    endDate.setHours(23, 59, 59, 999);
    startDate = new Date(endDate);
    startDate.setDate(endDate.getDate() - 7);
    startDate.setHours(0, 0, 0, 0);
  }

  return { startDate, endDate };
}

// Function to calculate new date ranges for pagination
function calculateDateRangeUrls() {
  const { startDate, endDate } = calculateStartEndDate();
  const rangeDays = Math.ceil((endDate - startDate) / (1000 * 60 * 60 * 24));

  const olderStart = new Date(startDate);
  olderStart.setDate(startDate.getDate() - rangeDays);
  const olderEnd = new Date(endDate);
  olderEnd.setDate(endDate.getDate() - rangeDays);

  const newerStart = new Date(startDate);
  newerStart.setDate(startDate.getDate() + rangeDays);
  const newerEnd = new Date(endDate);
  newerEnd.setDate(endDate.getDate() + rangeDays);

  const olderUrl = `./?startDate=${yyyymmdd(olderStart)}&endDate=${yyyymmdd(olderEnd)}`;
  const newerUrl = `./?startDate=${yyyymmdd(newerStart)}&endDate=${yyyymmdd(newerEnd)}`;

  return { olderUrl, newerUrl };
}

// Function to update date range text and pagination URLs
function updateDateRangeAndLinks() {
  const params = new URLSearchParams(window.location.search);
  const startDate = parseDate(params.get("startDate"));
  const endDate = parseDate(params.get("endDate"));

  const dateRangeElement = document.querySelector("#date-range");
  if (dateRangeElement) {
    const displayStartDate = new Date(startDate);
    displayStartDate.setDate(displayStartDate.getDate() + 1);
    const displayEndDate = new Date(endDate);
    displayEndDate.setDate(displayEndDate.getDate() + 1);
    dateRangeElement.textContent = `Searching Files from ${yyyymmdd(displayStartDate)} to ${yyyymmdd(displayEndDate)}`;
  }

  // Update pagination link URLs based on original params or defaults
  const { olderUrl, newerUrl } = calculateDateRangeUrls();
  const olderLink = document.querySelector("#older-link");
  const newerLink = document.querySelector("#newer-link");
  if (olderLink) olderLink.setAttribute("data-url", olderUrl);
  if (newerLink) newerLink.setAttribute("data-url", newerUrl);
}

// Function to populate the accordion (unchanged)
function populateAccordion() {
  const accordionContainer = document.querySelector("#accordion-container");
  if (!accordionContainer) return;

  predefinedQueries.forEach((category, index) => {
    const accordionItem = document.createElement("div");
    accordionItem.classList.add("accordion-item");

    const header = document.createElement("button");
    header.classList.add("accordion-header");
    header.textContent = category.category;
    header.setAttribute("aria-expanded", "false");
    header.setAttribute("aria-controls", `accordion-content-${index}`);

    const content = document.createElement("div");
    content.classList.add("accordion-content");
    content.id = `accordion-content-${index}`;

    category.queries.forEach((query) => {
      const option = document.createElement("button");
      option.classList.add("query-option");
      option.textContent = query.name;
      option.setAttribute("data-query", query.query.trim());
      option.addEventListener("click", () => {
        const queryInput = document.querySelector("#query");
        if (queryInput) queryInput.value = query.query.trim();
      });
      content.appendChild(option);
    });

    header.addEventListener("click", () => {
      const isActive = content.classList.contains("active");
      document.querySelectorAll(".accordion-content").forEach((c) => {
        c.classList.remove("active");
        c.previousElementSibling.setAttribute("aria-expanded", "false");
      });
      if (!isActive) {
        content.classList.add("active");
        header.setAttribute("aria-expanded", "true");
      }
    });

    accordionItem.appendChild(header);
    accordionItem.appendChild(content);
    accordionContainer.appendChild(accordionItem);
  });
}

// Main initialization
window.onload = function () {
  populateAccordion();
  updateDateRangeAndLinks();

  let isInitialLoad = true;

  const olderLink = document.querySelector("#older-link");
  const newerLink = document.querySelector("#newer-link");

  if (olderLink) {
    olderLink.addEventListener("click", (event) => {
      event.preventDefault();
      const newUrl = olderLink.getAttribute("data-url");
      window.history.pushState(
        {
          startDate: new URLSearchParams(newUrl).get("startDate"),
          endDate: new URLSearchParams(newUrl).get("endDate"),
        },
        "",
        newUrl
      );
      updateDateRangeAndLinks();
    });
  }

  if (newerLink) {
    newerLink.addEventListener("click", (event) => {
      event.preventDefault();
      const newUrl = newerLink.getAttribute("data-url");
      window.history.pushState(
        {
          startDate: new URLSearchParams(newUrl).get("startDate"),
          endDate: new URLSearchParams(newUrl).get("endDate"),
        },
        "",
        newUrl
      );
      updateDateRangeAndLinks();
    });
  }

  window.addEventListener("popstate", () => {
    if (isInitialLoad) {
      isInitialLoad = false;
      return;
    }
    updateDateRangeAndLinks();
  });

  const searchForm = document.querySelector("#search-form");
  if (searchForm) {
    searchForm.addEventListener("submit", function (event) {
      event.preventDefault();
      const query = document.querySelector("#query");
      if (query) {
        performSearch(query.value);
      }
    });
  }
};

function encodeSQLToBase64(sql) {
  return btoa(sql);
}

function performSearch(body) {
  const params = new URLSearchParams(window.location.search);
  const queryParams = new URLSearchParams();
  queryParams.set("startDate", params.get("startDate") || yyyymmdd(new Date()));
  queryParams.set("endDate", params.get("endDate") || yyyymmdd(new Date()));
  const patternElm = document.querySelector("#pattern");
  if (patternElm) {
    queryParams.set("pattern", patternElm.value);
  }

  const errorElm = document.querySelector("#error");
  if (errorElm) {
    errorElm.textContent = "";
  }

  fetch(`./search?${queryParams.toString()}`, {
    method: "POST",
    headers: {
      "Content-Type": "text/plain",
    },
    body: JSON.stringify({
      query: encodeSQLToBase64(body),
    }),
  })
    .then((response) => response.json())
    .then((data) => {
      populateSearchResponse(data);
    })
    .catch((error) => {
      const errorElm = document.querySelector("#error");
      if (errorElm) {
        errorElm.textContent = error.message;
      } else {
        console.error("Error element (#error) not found");
      }
      console.error("Fetch error:", error);
    });
}

function populateSearchResponse(data) {
  const table = document.querySelector("#results");
  if (table) {
    table.innerHTML = "";

    const headers = document.createElement("tr");
    if (data.Headers) {
      data.Headers.Columns.forEach((col) => {
        const th = document.createElement("th");
        th.innerHTML = col;
        headers.append(th);
      });
      table.append(headers);
    }

    if (data.Rows) {
      data.Rows.forEach((r) => {
        const row = document.createElement("tr");
        r.Columns.forEach((col) => {
          const td = document.createElement("td");
          td.innerHTML = col;
          row.append(td);
        });
        table.append(row);
      });
    }

    if (data.error) {
      const errorElm = document.querySelector("#error");
      if (errorElm) {
        errorElm.textContent = data.error;
      }
    }
  }
}
