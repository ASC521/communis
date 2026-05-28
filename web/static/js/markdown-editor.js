const mdeTextareaID = "mde-textarea"
const mdePreviewID = "mde-preview"
const mdeRENewLine = /\n/;
const mdeREWord = /[a-zA-Z0-9\-']/;

let mdEditor = null;

class MDEditor {
    history = [];
    currentIndex = 0;
    isUndoRedo = false;
    isDirty = false;
    maxHistory = 10;
    saveTimeout = null;
    textarea = null;
    container = null;
    saveBtnClicked = false;
    deleteBtnClicked = false;

    constructor(textarea, container) {
	this.textarea = textarea;
	this.history.push(textarea.value);
	this.container = container;
    }
    
}

function mdeUndo(mde) {
    if (mde.currentIndex <= 0) {
	return
    }
    
    mde.currentIndex--;
    mdeRestoreState(mde);
}

function mdeRedo(mde) {
    if (mde.currentIndex == mde.history.length-1) {
	return
    }

    mde.currentIndex++;
    mdeRestoreState(mde);
}

function mdeRestoreState(mde) {
    mde.isUndoRedo = true;
    
    const cursorPos = mde.textarea.selectionStart;
    mde.textarea.value = mde.history[mde.currentIndex]

    const newPos = Math.min(cursorPos, mde.textarea.value.length);
    mde.textarea.setSelectionRange(newPos, newPos);

    mde.isUndoRedo = false;
}

function mdeAddHistoryEntry(mde) {
    if (mde.isUndoRedo) {
	return
    }

    mde.history = mde.history.slice(0, mde.currentIndex + 1);
    mde.history.push(mde.textarea.value);
    mde.currentIndex++;

    if (mde.history.length > mde.maxHistory) {
	mde.history.shift();
	mde.currentIndex--;
    }
}

function mdeAddUndoRedoEntry(event) {
    if (mdEditor.isUndoRedo) {
	return
    }
    
    clearTimeout(mdEditor.saveTimeout);
    mdEditor.saveTimeout = setTimeout(() => {
	mdeAddHistoryEntry(mdEditor);
    }, 500);
}

function mdeHandleOnInput(event) {
    mdeAddUndoRedoEntry(event);
}

function mdeHandleKeyDown(event) {

    let u = null;
    if (event.ctrlKey || event.metaKey) {
	switch (event.key) {
	case "b":
	    event.preventDefault();
	    u = mdeInsertWrap(mdEditor.textarea, "**");
	    break;
	case "i":
	    event.preventDefault();
	    u = mdeInsertWrap(mdEditor.textarea, "*");
	    break;
	case "k":
	    event.preventDefault();
	    u = mdeInsertLink(mdEditor.textarea);
	    break;
	case "z":
	    event.preventDefault();
	    mdeUndo(mdEditor);
	    break;
	case "y":
	    event.preventDefault();
	    mdeRedo(mdEditor);
	default:
	    return
	}
    }

    if (u === null) {
	console.log("update is null");
	return
    }

    mdEditor.textarea.value = u.value;
    mdEditor.textarea.setSelectionRange(u.selectionStart, u.selectionEnd);
    mdeAddHistoryEntry(mdEditor);
    mdEditor.textarea.focus();
}

function mdeMonitorChanges(event) {
    mdEditor.isDirty = true;
}

function mdeConfirmChangesLoss(event) {
    if (!mdEditor.isDirty || mdEditor.saveBtnClicked || mdEditor.deleteBtnClicked) {
	return
    }
    
    event.preventDefault()
}


class MDEValueUpdate {
    value;
    selectionStart;
    selectionEnd;

    constructor(value, start, end) {
	this.value = value;
	this.selectionStart = start;
	this.selectionEnd = end;
    }
}


function mdeInsertWrap(textarea, chars) {
    
    let start = textarea.selectionStart
    let end = textarea.selectionEnd
    const taContent = textarea.value

    if (start === end) {
	while (start > 0 && mdeREWord.test(taContent[start-1])) {
	    start--;
	}

	while (end < taContent.length && mdeREWord.test(taContent[end])) {
	    end++;
	}

    }
    const left = taContent.slice(0, start);
    const mid = taContent.slice(start, end);
    const right = taContent.slice(end);

    const cursorPos = textarea.selectionEnd + chars.length;
    const value =   left + chars + mid + chars + right

    return new MDEValueUpdate(value, cursorPos, cursorPos);
}


function mdeInsertBeginningOfLine(textarea, chars, insertCount) {

    let start = textarea.selectionStart
    let end = textarea.selectionEnd
    const taContent = textarea.value

    const charsHighlighted = (start !== end);
    if (!charsHighlighted) {
	while (start > 0) {
	    let c = taContent[start-1]
	    if (mdeRENewLine.test(c)) {
		break;
	    }
	    start--;
	}
	const left = taContent.slice(0, start);
	const right = taContent.slice(start);

	const cursorPos = textarea.selectionEnd + chars.length;
	const value =   left + chars + right
	return new MDEValueUpdate(value, cursorPos, cursorPos)
	
    } else {

	while (start > 0) {
	    let c = taContent[start];
	    if (mdeRENewLine.test(c)) {
		break;
	    }
	    start--;
	}
	
	const left = taContent.slice(0, start);
	let mid = taContent.slice(start, end);
	const right = taContent.slice(end);

	let count = 0;
	let charsInserted = 0;
	mid = mid.replaceAll(/\n/g, (match) => {
	    count++;

	    if (insertCount) {
		let sc = count.toString() + ".";
		charsInserted += sc.length + chars.length;
		return "\n" + sc + chars;
	    } else {
		charsInserted += chars.length;
		return "\n"+chars;
	    }
	    

	});

	const cursorPos = textarea.selectionEnd + charsInserted;
	const value = left + mid + right;
	return new MDEValueUpdate(value, cursorPos, cursorPos); 
    }
}


function mdeInsertLink(textarea) {
    
    let start = textarea.selectionStart
    let end = textarea.selectionEnd
    const taContent = textarea.value

    const charsHighlighted = (start !== end);
    if (!charsHighlighted) {
	while (start > 0 && mdeREWord.test(taContent[start-1])) {
	    start--;
	}

	while (end < taContent.length && mdeREWord.test(taContent[end])) {
	    end++;
	}

    }
    const left = taContent.slice(0, start);
    const mid = taContent.slice(start, end);
    const right = taContent.slice(end);

    if (charsHighlighted) {
	const cursorPos = start + mid.length + 3;
	const value =   left + "[" + mid + "]()" + right
	return new MDEValueUpdate(value, cursorPos, cursorPos);
    } else {
	const cursorPosStart = start + mid.length + 3;
	const cursorPosEnd = cursorPosStart + 3;
	const value =   left + "[" + mid + "](url)" + right
	return new MDEValueUpdate(value, cursorPosStart, cursorPosEnd);
    }
}


function mdeInsertCodeBlock(textarea) {
    
    let start = textarea.selectionStart
    let end = textarea.selectionEnd
    const taContent = textarea.value

    const charsHighlighted = (start !== end);
    if (!charsHighlighted) {
	while (start > 0) {
	    // check the next character since we want to start at the character
	    // before the one we are searching for
	    if (mdeRENewLine.test(taContent[start-1])) {
		break;
	    }
	    start--;

	}

	while (end < mdeRENewLine.length && mdeREWord.test(taContent[end])) {
	    end++;
	}

    }
    const left = taContent.slice(0, start);
    const mid = taContent.slice(start, end);
    const right = taContent.slice(end);

    const pre = "```\n";
    const post = "\n```";
    const cursorPos = start + mid.length + pre.length;
    const value =   left + pre + mid + post + right;
    return new MDEValueUpdate(value, cursorPos, cursorPos)
}


function mdeTogglePreview() {

    const form = document.querySelector("#note-editor");
    if (form === null) {
	return
    }

    const previewDiv = document.querySelector("#" + mdePreviewID);
    if (previewDiv === null) {
	return
    }

    if (form.style.display === "none") {
	form.style.display = "";
	previewDiv.hidden = true;
    } else {
	form.style.display = "none"
	previewDiv.hidden = false;
    }
    
}


function mdeHandleClick(event) {

    const buttonID = event.target.closest("button")?.id;
    if (buttonID === null) {
	return
    }
    
    let u = null;
    switch (buttonID) {
    case "mde-preview-btn":
	mdeTogglePreview();
	break;
    case "mde-bold-btn":
	u = mdeInsertWrap(mdEditor.textarea, "**");
	break;
    case "mde-italic-btn":
	u = mdeInsertWrap(mdEditor.textarea, "*");
	break;
    case "mde-inline-code-btn":
	u = mdeInsertWrap(mdEditor.textarea, "`");
	break;
    case "mde-h1-btn":
	u = mdeInsertBeginningOfLine(mdEditor.textarea, "# ", false);
	break;
    case "mde-h2-btn":
	u = mdeInsertBeginningOfLine(mdEditor.textarea, "## ", false);
	break;
    case "mde-h3-btn":
	u = mdeInsertBeginningOfLine(mdEditor.textarea, "### ", false);
	break;
    case "mde-quote-btn":
	u = mdeInsertBeginningOfLine(mdEditor.textarea, "> ", false);
	break;
    case "mde-link-btn":
	u = mdeInsertLink(mdEditor.textarea);
	break;
    case "mde-code-block-btn":
	u = mdeInsertCodeBlock(mdEditor.textarea);
	break;
    case "mde-unordered-list-button":
	u = mdeInsertBeginningOfLine(mdEditor.textarea, "* ", false);
	break;
    case "mde-ordered-list-button":
	u = mdeInsertBeginningOfLine(mdEditor.textarea, " ", true);
	break;
    case "mde-undo-btn":
	mdeUndo(mdEditor);
	break;
    case "mde-redo-btn":
	mdeRedo(mdEditor);
	break;
    case "mde-save-btn":
	mdEditor.saveBtnClicked = true;
    case "mde-delete-btn":
	mdEditor.deleteBtnClicked = true;
    default:
	return
    }

    if (u === null) {
	console.log("markdown editor update is null")
	return
    }

    mdEditor.textarea.value = u.value;
    mdEditor.textarea.setSelectionRange(u.selectionStart, u.selectionEnd);
    mdeAddHistoryEntry(mdEditor);
    mdEditor.textarea.focus();
}

document.addEventListener("DOMContentLoaded", (e) => {

    const container = document.querySelector("#mde-container");
    const ta = document.querySelector("#" + mdeTextareaID);

    if (container === null) {
	if (mdEditor !== null) {
	    mdEditor.textarea.removeEventListener("input", mdeHandleOnInput);
	    mdEditor.textarea.removeEventListener("keydown", mdeHandleKeyDown);
	    mdEditor.container.removeEventListener("click", mdeHandleClick);
	    mdEditor.container.removeEventListener("change", mdeMonitorChanges)
	    removeEventListener("beforeunload", mdeConfirmChangesLoss)
	    mdEditor = null;
	}
	return
    }

    if (mdEditor === null) {
	mdEditor = new MDEditor(ta, container);
	// mdeAdjustTextareaHeight();
	mdEditor.textarea.addEventListener("input", mdeHandleOnInput);
	mdEditor.textarea.addEventListener("keydown", mdeHandleKeyDown);
	mdEditor.container.addEventListener("click", mdeHandleClick);
	mdEditor.container.addEventListener("change", mdeMonitorChanges)
	addEventListener("beforeunload", mdeConfirmChangesLoss)
    }
    
});
