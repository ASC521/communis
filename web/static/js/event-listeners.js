import OverType, { toolbarButtons } from '../vendor/js/overtype_v2-1-0.esm.js';

function initEditorIfThere() {
    const editorDiv =document.querySelector("#overtype-editor") 
    if (editorDiv != null) {
        const [editor] =  new OverType('#overtype-editor', {
	    value: editorDiv.dataset.initValue,
	    toolbar: true,
	    toolbarButtons: [
		toolbarButtons.bold,
		toolbarButtons.italic,
		toolbarButtons.code,
		toolbarButtons.link,
		toolbarButtons.h1,
		toolbarButtons.h2,
		toolbarButtons.h3,
		toolbarButtons.bulletList,
		toolbarButtons.orderedList,
		toolbarButtons.taskList,
		toolbarButtons.quote,
		toolbarButtons.separator,
	    ],
	    textareaProps: {
		name: 'content'
	    },
	    theme: 'cave',
	    autoResize: true,
	});
    }
}

document.addEventListener('htmx:after:swap', function(evt) {
    initEditorIfThere();
});

document.addEventListener('DOMContentLoaded', function(evt) {
    initEditorIfThere();
});
