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
  if (!date || isNaN(date)) return "Unknown";
  const year = date.getFullYear();
  const month = String(date.getMonth() + 1).padStart(2, "0");
  const day = String(date.getDate()).padStart(2, "0");
  return `${year}-${month}-${day}`;
}

function calculateStartEndDate() {
  const params = new URLSearchParams(window.location.search);
  let startDate = params.get("startDate") ? new Date(params.get("startDate")) : null;
  let endDate = params.get("endDate") ? new Date(params.get("endDate")) : null;

  // Default to a 7-day range ending today if dates are invalid or missing
  if (!startDate || !endDate || isNaN(startDate) || isNaN(endDate)) {
    endDate = new Date(); // Today
    startDate = new Date();
    startDate.setDate(endDate.getDate() - 7); // 7 days before today
  }

  return { startDate, endDate };
}

// Function to calculate new date ranges for pagination
function calculateDateRangeUrls() {
  const { startDate, endDate } = calculateStartEndDate();

  // Calculate "Older" range (shift back 7 days)
  const olderStart = new Date(startDate);
  olderStart.setDate(startDate.getDate() - 7);
  const olderEnd = new Date(endDate);
  olderEnd.setDate(endDate.getDate() - 7);

  // Calculate "Newer" range (shift forward 7 days)
  const newerStart = new Date(startDate);
  newerStart.setDate(startDate.getDate() + 7);
  const newerEnd = new Date(endDate);
  newerEnd.setDate(endDate.getDate() + 7);

  // Generate URLs
  const olderUrl = `./?startDate=${yyyymmdd(olderStart)}&endDate=${yyyymmdd(olderEnd)}`;
  const newerUrl = `./?startDate=${yyyymmdd(newerStart)}&endDate=${yyyymmdd(newerEnd)}`;

  return { olderUrl, newerUrl };
}

// Function to update date range text and pagination URLs
function updateDateRangeAndLinks() {
  const { startDate, endDate } = calculateStartEndDate();

  const dateRangeElement = document.querySelector("#date-range");
  dateRangeElement.textContent = `Searching Files from ${yyyymmdd(startDate)} to ${yyyymmdd(endDate)}`;

  // Update pagination link URLs
  const { olderUrl, newerUrl } = calculateDateRangeUrls();
  document.querySelector("#older-link").setAttribute("data-url", olderUrl);
  document.querySelector("#newer-link").setAttribute("data-url", newerUrl);
}

// Function to populate the accordion
function populateAccordion() {
  const accordionContainer = document.querySelector("#accordion-container");

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
        document.querySelector("#query").value = query.query.trim();
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
  // Populate the accordion
  populateAccordion();

  // Update date range text and pagination URLs on load
  updateDateRangeAndLinks();

  // Handle pagination link clicks
  const olderLink = document.querySelector("#older-link");
  const newerLink = document.querySelector("#newer-link");

  olderLink.addEventListener("click", (event) => {
    event.preventDefault();
    const newUrl = olderLink.getAttribute("data-url");
    window.history.pushState({ startDate: new URLSearchParams(newUrl).get("startDate"), endDate: new URLSearchParams(newUrl).get("endDate") }, "", newUrl);
    updateDateRangeAndLinks();
  });

  newerLink.addEventListener("click", (event) => {
    event.preventDefault();
    const newUrl = newerLink.getAttribute("data-url");
    window.history.pushState({ startDate: new URLSearchParams(newUrl).get("startDate"), endDate: new URLSearchParams(newUrl).get("endDate") }, "", newUrl);
    updateDateRangeAndLinks();
  });

  // Handle browser back/forward navigation
  window.addEventListener("popstate", () => {
    updateDateRangeAndLinks();
  });

  // Handle search form submission
  const searchForm = document.querySelector("#search-form");
  searchForm.addEventListener("submit", function (event) {
    event.preventDefault();
    const query = document.querySelector("#query");
    performSearch(query.value);
  });
};

function encodeSQLToBase64(sql) {
    return btoa(sql);
}

function performSearch(body) {
  const currentParams = new URLSearchParams(window.location.search);
  const queryParams = new URLSearchParams();

  // Set query params
  const { startDate, endDate } = calculateStartEndDate();
  if (startDate != null) {
    queryParams.set("startDate", yyyymmdd(startDate));
  }
  if (endDate != null) {
    queryParams.set("endDate", yyyymmdd(endDate));
  }
  var patternElm = document.querySelector("#pattern");
  if (patternElm) {
    queryParams.set("pattern", patternElm.value);
  }

  // Clear error message before search
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
      const elm = document.querySelector("#error");
      if (elm) {
        elm.textContent = error.message;
      } else {
        console.error("Error element (#error) not found");
      }
      console.error("Fetch error:", error);
    });
}

function populateSearchResponse(data) {
  const table = document.querySelector("#results");
  table.innerHTML = ""; // Clear the previous rows

  // Add the Headers
  const headers = document.createElement("tr");
  if (data.Headers) {
    data.Headers.Columns.forEach((col) => {
      const th = document.createElement("th");
      th.innerHTML = col;
      headers.append(th);
    });
    table.append(headers);
  }

  // Add the rows
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

  // Show an error if one exists
  if (data.error) {
    const elm = document.querySelector("#error");
    elm.textContent = data.error;
  }
}
