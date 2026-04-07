// THEME SWITCHER
const preferDarkTheme = window.matchMedia("(prefers-color-scheme: dark)").matches;

function toggleTheme() {
    const h = document.querySelector("html");
    const ct = h.getAttribute("data-theme");
    let t = "light"
    if (ct === "auto" && preferDarkTheme) {
	t = "dark"
    } else if (ct === "light" ) {
	t = "dark"
    }

    h.setAttribute("data-theme", t);
}


document.addEventListener("click", (e) => {
    if (e.target.id === "theme-toggle-btn") {
	e.preventDefault();
	toggleTheme();
	const nd = document.querySelector(".dropdown details");
	nd.removeAttribute("open");
    }
})


// THEME SWITCHER

// TAG FILTER

function filterFieldSet(id, filter) {
    const fs = document.querySelector(id);
    if (fs === null) {
	console.log("no fieldset found for id ${id}");
	return
    }

    const searchTerm = filter.toLowerCase();
    const fields = fs.querySelectorAll('input');

    fields.forEach(field => {
	const value = field.value.toLowerCase();
	const label = field.labels?.[0]

	if (label !== null) {
	    const labelText = label.textContent.toLowerCase();
	    if (labelText.includes(searchTerm) || searchTerm === '') {
		field.style.display = '';
		label.style.display = '';
	    } else {
		field.style.display = 'none';
		label.style.display = 'none';
	    }
	}
	
    });
}


document.addEventListener('input', (e) => {
    if (e.target.id === 'tag-name') {
	filterFieldSet('#tag-selector', e.target.value)
    }
});

// TAG FILTER

// SECTION FORM REMOVAL

function removeInlineSectionForm(button) {

    const form = button.closest("#inline-section-form");
    if (form === null) {
	return
    }

    form.remove();
    
}

document.addEventListener("click", (e) => {

    const button = event.target.closest("button");
    if (button === null) {
	return
    }

    
    if (button.id == "btn-remove-section-form") {
	removeInlineSectionForm(button);
    }
});

// SECTION FORM REMOVAL

// DATETIME FORMAT

function convertToLocalTime() {
    document.querySelectorAll('time.local-time').forEach(function(timeEl) {
    const utcTime = new Date(timeEl.getAttribute('datetime'));
    timeEl.textContent = utcTime.toLocaleString('en-US', {
      year: 'numeric',
      month: 'short',
      day: 'numeric',
      hour: '2-digit',
      minute: '2-digit',
      timeZoneName: 'short'
    });
  });  
}

document.addEventListener('DOMContentLoaded', convertToLocalTime);
document.addEventListener('htmx:after:swap', convertToLocalTime)

// DATETIME FORMAT

// DELETE CONTAINING TABLE ROW

document.addEventListener("click", (e) => {
    if (e.target.id === "remove-table-row") {
	e.target.closest("tr").remove();
    }

});

// DELETE CONTAINING TABLE ROW

// SELECT REFERENCE NOTES

document.addEventListener("click", (e) => {
    if (e.target.id === "ref-note-btn") {
	selectedNotes = document.querySelector('#selected-notes');
	if (selectedNotes === null) {
	    console.log("failed to find selected notes field set");
	    return
	}

	input = document.createElement("input");
	input.type = "checkbox";
	input.name = "ref-notes";
	input.id = "ref-note-" + e.target.dataset.noteId;
	input.checked = true;

	label = document.createElement("label");
	label.innerHTML = e.target.dataset.noteTitle;
	
	selectedNotes.append(input, label);
    }

    if (e.currentTarget.activeElement.id == "sidebar-toggle") {
	sidebar = document.querySelector('#' + e.currentTarget.activeElement.dataset.sidebarId);
	if (sidebar === null) {
	    console.log("sidebar not found");
	    return
	}

	if (sidebar.classList.contains('-closed')) {
	    sidebar.classList.remove('-closed');
	    e.currentTarget.activeElement.classList.add('-active');	    
	} else {
	    sidebar.classList.add('-closed');
	    e.currentTarget.activeElement.classList.remove('-active');	    
	}	
    }
});

// SELECT REFERENCE NOTES
