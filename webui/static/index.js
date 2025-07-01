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
    ],
  },
  {
    category: "Exceptions",
    queries: [
      { name: "Recent Returns and Corrections", query: `
SELECT
  -- e.entry_id,
  e.individual_name, e.dfi_account_number, e.amount,
  e.trace_number, a.return_code,
  -- a.payment_related_information,
  f.filename, f.file_creation_date

FROM ach_entries e
JOIN ach_addendas a ON e.entry_id = a.entry_id AND e.file_id = a.file_id
JOIN ach_files f ON e.file_id = f.file_id

WHERE a.return_code IS NOT NULL OR a.change_code IS NOT NULL

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

// Function to calculate new date ranges for pagination
function calculateDateRangeUrls() {
  const params = new URLSearchParams(window.location.search);
  let startDate = params.get("startDate") ? new Date(params.get("startDate")) : new Date();
  let endDate = params.get("endDate") ? new Date(params.get("endDate")) : new Date();

  // Default to a 7-day range if dates are invalid
  if (isNaN(startDate) || isNaN(endDate)) {
    endDate = new Date();
    startDate = new Date();
    startDate.setDate(endDate.getDate() - 7);
  }

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
  const params = new URLSearchParams(window.location.search);
  const startDate = params.get("startDate") ? new Date(params.get("startDate")) : new Date();
  const endDate = params.get("endDate") ? new Date(params.get("endDate")) : new Date();
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
      option.setAttribute("data-query", query.query);
      option.addEventListener("click", () => {
        document.querySelector("#query").value = query.query;
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

function performSearch(body) {
  const currentParams = new URLSearchParams(window.location.search);
  const queryParams = new URLSearchParams();
  const startDate = currentParams.get("startDate");
  if (startDate != null) {
    queryParams.set("startDate", startDate);
  }
  const endDate = currentParams.get("endDate");
  if (endDate != null) {
    queryParams.set("endDate", endDate);
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
    body: body,
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
