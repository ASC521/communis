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

document.addEventListener('change', (e) => {
    if (e.target.type === 'checkbox') {
	const fs = e.target.closest('fieldset');
	if (fs.id !== 'tag-selector') {
	    return
	}

	const count = fs.querySelectorAll('input[type="checkbox"]:checked').length;

	const countSpan = document.querySelector('#checked-tag-count');
	if (countSpan === null) {
	    return
	}
	countSpan.textContent = count;
    }
});

