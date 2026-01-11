const mdeTextareaID = "mde-textarea"
const mdeRENewLine = /\n/;
const mdeREWord = /[a-zA-Z0-9\-']/;

const mdeHistoryInstances = new WeakMap()

class MDEHistory {
    entries = [];
    currentIndex = 0;
    isUndoRedo = false;
    maxEntries = 10;
    saveTimeout = null;

    constructor(value) {
	this.entries.push(value);
    }
    
}

function mdeUndo(history, textarea) {
    if (history.currentIndex <= 0) {
	return
    }
    
    history.currentIndex--;
    mdeRestoreState(history, textarea);
}

function mdeRedo(history, textarea) {
    if (history.currentIndex == history.entries.length-1) {
	return
    }

    history.currentIndex++;
    mdeRestoreState(history, textarea);
}

function mdeRestoreState(history, textarea) {
    history.isUndoRedo = true;
    
    const cursorPos = textarea.selectionStart;
    textarea.value = history.entries[history.currentIndex]

    const newPos = Math.min(cursorPos, textarea.value.length);
    textarea.setSelectionRange(newPos, newPos);

    history.isUndoRedo = false;
}

function mdeAddHistoryEntry(history, textarea) {
    if (history.isUndoRedo) {
	return
    }

    history.entries = history.entries.slice(0, history.currentIndex + 1);
    history.entries.push(textarea.value);
    history.currentIndex++;

    if (history.entries.length > history.maxEntries) {
	history.entries.shift();
	history.currentIndex--;
    }
}

function mdeHistoryHandleOnInput(event) {
    const textarea = event.target

    let h = null;
    if (mdeHistoryInstances.has(textarea)) {
	h = mdeHistoryInstances.get(textarea);
    } else {
	h = new MDEHistory(textarea);
	mdeHistoryInstances.set(textarea, h);
    }

    if (h.isUndoRedo) {
	return
    }
    
    clearTimeout(h.saveTimeout);
    h.saveTimeout = setTimeout(() => {
	mdeAddHistoryEntry(h, textarea);
    }, 500);

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

document.addEventListener("click", (e) => {
    const ta = document.querySelector("#" + mdeTextareaID);
    if (ta === null) {
	console.log("markdown editor textareanot found in document");
	return
    }

    let h = null;
    if (mdeHistoryInstances.has(ta)) {
	h = mdeHistoryInstances.get(ta);
    } else {
	h = new MDEHistory(ta.value);
	mdeHistoryInstances.set(ta, h);
    }

    let u = null;
    switch (e.target.id) {
    case "mde-bold-btn":
	u = mdeInsertWrap(ta, "**");
	break;
    case "mde-italic-btn":
	u = mdeInsertWrap(ta, "*");
	break;
    case "mde-inline-code-btn":
	u = mdeInsertWrap(ta, "`");
	break;
    case "mde-h1-btn":
	u = mdeInsertBeginningOfLine(ta, "# ", false);
	break;
    case "mde-h2-btn":
	u = mdeInsertBeginningOfLine(ta, "## ", false);
	break;
    case "mde-h3-btn":
	u = mdeInsertBeginningOfLine(ta, "### ", false);
	break;
    case "mde-quote-btn":
	u = mdeInsertBeginningOfLine(ta, "> ", false);
	break;
    case "mde-link-btn":
	u = mdeInsertLink(ta);
	break;
    case "mde-code-block-btn":
	u = mdeInsertCodeBlock(ta);
	break;
    case "mde-unordered-list-button":
	u = mdeInsertBeginningOfLine(ta, "* ", false);
	break;
    case "mde-ordered-list-button":
	u = mdeInsertBeginningOfLine(ta, " ", true);
	break;
    case "mde-undo-btn":
	mdeUndo(h, ta);
	break;
    case "mde-redo-btn":
	mdeRedo(h, ta);
	break;
    default:
	return
    }

    if (u === null) {
	console.log("markdown editor update is null")
	return
    }

    ta.value = u.value;
    ta.setSelectionRange(u.selectionStart, u.selectionEnd);
    mdeAddHistoryEntry(h, ta);
    ta.focus();
});

document.addEventListener("keydown", (e) => {
    if (e.target.id !== mdeTextareaID) {
	return
    }

    const ta = document.querySelector("#" + mdeTextareaID);
    if (ta === null) {
	console.log("markdown editor textarea not found in document");
	return
    }

    let h = null;
    if (mdeHistoryInstances.has(ta)) {
	h = mdeHistoryInstances.get(ta);
    } else {
	h = new MDEHistory(ta.value);
	mdeHistoryInstances.set(ta, h);
    }

    let u = null;
    if (e.ctrlKey || e.metaKey) {
	switch (e.key) {
	case "b":
	    e.preventDefault();
	    u = mdeInsertWrap(ta, "**");
	    break;
	case "i":
	    e.preventDefault();
	    u = mdeInsertWrap(ta, "*");
	    break;
	case "k":
	    e.preventDefault();
	    u = mdeInsertLink(ta);
	    break;
	case "z":
	    e.preventDefault();
	    mdeUndo(h, ta);
	    break;
	case "y":
	    e.preventDefault();
	    mdeRedo(h, ta);
	default:
	    return
	}
    }

    if (u === null) {
	console.log("update is null");
	return
    }

    ta.value = u.value;
    ta.setSelectionRange(u.selectionStart, u.selectionEnd);
    mdeAddHistoryEntry(h, ta);
    ta.focus();
});

document.addEventListener("htmx:after:init", (e) => {
    // console.log("htmx:after:init - ", e);
    if (e.target.id !== "note-editor") {
	return
    }
    
    const ta = document.querySelector("#" + mdeTextareaID);
    if (ta === null || mdeHistoryInstances.has(ta)) {
	return
    }

    ta.addEventListener("input", mdeHistoryHandleOnInput);
});


document.addEventListener("htmx:before:cleanup", (e) => {
    // TODO: htmx docs say object remove is stored in
    // event.detail.elt but that object is always empty
    // recheck once htmx v6 goes stable
    // console.log("htmx:before:cleanup", e);
    if (e.target.id !== "note-editor") {
	return
    }

    const ta = document.querySelector("#" + mdeTextareaID);
    if (ta.id === null) {
	return
    }

    if (!mdeHistoryInstances.has(ta)) {
	return
    }

    ta.removeEventListener("input", mdeHistoryHandleOnInput);
    mdeHistoryInstances.delete(ta);
});

