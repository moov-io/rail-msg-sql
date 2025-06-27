window.onload = function() {
  var searchForm = document.querySelector("#search-form");

  searchForm.addEventListener('submit', function(event) {
    event.preventDefault();

    var query = document.querySelector("#query");
    performSearch(query.value);
  });
};

function performSearch(body) {
  const currentParams = new URLSearchParams(window.location.search);

  const queryParams = new URLSearchParams();
  var startDate = currentParams.get('startDate');
  if (startDate != null) {
    queryParams.set("startDate", startDate);
  }
  var endDate = currentParams.get('endDate');
  if (endDate != null) {
    queryParams.set("endDate", endDate);
  }

  fetch(`./search?${queryParams.toString()}`, {
    method: 'POST',
    headers: {
      'Content-Type': 'text/plain'
    },
    body: body,
  })
    .then(response => response.json())
    .then(data => {
      populateSearchResponse(data)
    })
    .catch(error => {
      var elm = document.querySelector("#error");
      if (elm) {
        elm.textContent = error.message;
      } else {
        console.error('Error element (#error) not found');
      }
      console.error('Fetch error:', error);
    });
}

function populateSearchResponse(data) {
  var table = document.querySelector("#results");
  table.innerHTML = ''; // Clear the previous rows

  // Add the Headers
  const headers = document.createElement("tr"); // Fixed typo
  if (data.Headers) {
    data.Headers.Columns.forEach(col => {
      const th = document.createElement("th"); // Use <th> for headers
      th.innerHTML = col;

      headers.append(th);
    });
    table.append(headers);
  }

  // Add the rows
  if (data.Rows) {
    data.Rows.forEach(r => {
      const row = document.createElement("tr");

      r.Columns.forEach(col => {
        const td = document.createElement("td");
        td.innerHTML = col;

        row.append(td);
      });

      table.append(row);
    });
  }

  // Show an error if one exists
  if (data.error) {
    var elm = document.querySelector("#error");
    elm.textContent = data.error;
  }
}
