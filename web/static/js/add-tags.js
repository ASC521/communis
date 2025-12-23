document.querySelectorAll("[data-tags]")
    .forEach(el => {
	el.addEventListener('click', e => {
	    const el = e.srcElement

	    if (el.hasAttribute('data-tags-remove')) {
		el.parentElement.remove();
		return
	    }

	    if (el.hasAttribute('data-tags-add')) {
		const div = document.createElement('div');
		div.class = "input-group";

		const input = document.createElement('input');
		input.type = "text";
		input.name = "tags-list";
		input.placeHolder = "Tag " + (el.children.length + 1);
		div.appendChild(input);

		const button = document.createElement('button');
		button.type = "button";
		button.setAttribute('data-tags-remove', '');
		button.innerHTML = "X";
		div.appendChild(button);

		const tl = e.currentTarget.querySelector('[data-tags-list]');
		tl.appendChild(div);
	    }
	});
    });

