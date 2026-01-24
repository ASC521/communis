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
	const nd = document.querySelector("#navbar-dropdown");
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
