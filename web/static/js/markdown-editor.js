const mdeID = "mde-textarea"
const mdeRENewLine = /\n/;
const mdeREWord = /[a-zA-Z0-9\-']/;

function mdeInsertWrap(textareaID, chars) {
    const textarea = document.querySelector(textareaID);
    if (textarea === null) {
	console.log("textarea not found in document");
	return
    }
    
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
    textarea.value =   left + chars + mid + chars + right
    textarea.selectionEnd = cursorPos;
    textarea.focus();
}


function mdeInsertBeginningOfLine(textareaID, chars, insertCount) {
    const textarea = document.querySelector(textareaID);
    if (textarea === null) {
	console.log("textarea not found in document");
	return
    }

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
	textarea.value =   left + chars + right
	textarea.selectionEnd = cursorPos;
	textarea.focus();  
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
	textarea.value =   left + mid + right;
	textarea.selectionEnd = cursorPos;
	textarea.focus(); 
    }

  

}


function mdeInsertLink(textareaID) {
    const textarea = document.querySelector(textareaID);
    if (textarea === null) {
	console.log("textarea not found in document");
	return
    }
    
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
	textarea.value =   left + "[" + mid + "]()" + right
	textarea.selectionEnd = cursorPos;
    } else {
	const cursorPosStart = start + mid.length + 3;
	const cursorPosEnd = cursorPosStart + 3;
	textarea.value =   left + "[" + mid + "](url)" + right
	textarea.selectionStart = cursorPosStart;
	textarea.selectionEnd = cursorPosEnd;
    }
    
    textarea.focus();
}


function mdeInsertCodeBlock(textareaID) {
    const textarea = document.querySelector(textareaID);
    if (textarea === null) {
	console.log("textarea not found in document");
	return
    }
    
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
    textarea.value =   left + pre + mid + post + right;
    textarea.selectionEnd = cursorPos;
    
    textarea.focus();
}

document.addEventListener("click", (e) => {
    const qs = "#" + mdeID
    switch (e.target.id) {
    case "mde-bold-btn":
	mdeInsertWrap(qs, "**");
	break;
    case "mde-italic-btn":
	mdeInsertWrap(qs, "*");
	break;
    case "mde-inline-code-btn":
	mdeInsertWrap(qs, "`");
	break;
    case "mde-h1-btn":
	mdeInsertBeginningOfLine(qs, "# ", false);
	break;
    case "mde-h2-btn":
	mdeInsertBeginningOfLine(qs, "## ", false);
	break;
    case "mde-h3-btn":
	mdeInsertBeginningOfLine(qs, "### ", false);
	break;
    case "mde-quote-btn":
	mdeInsertBeginningOfLine(qs, "> ", false);
	break;
    case "mde-link-btn":
	mdeInsertLink(qs);
	break;
    case "mde-code-block-btn":
	mdeInsertCodeBlock(qs);
	break;
    case "mde-unordered-list-button":
	mdeInsertBeginningOfLine(qs, "* ", false);
	break;
    case "mde-ordered-list-button":
	mdeInsertBeginningOfLine(qs, " ", true);
	break;
    }
});

document.addEventListener("keydown", (e) => {
    if (e.target.id !== mdeID) {
	return
    }
    const qs = "#" + mdeID
    if (e.ctrlKey || e.metaKey) {
	switch (e.key) {
	case "b":
	    e.preventDefault();
	    mdeInsertWrap(qs, "**");
	    break;
	case "i":
	    e.preventDefault();
	    mdeInsertWrap(qs, "*");
	    break;
	case "k":
	    e.preventDefault();
	    mdeInsertLink(qs);
	    break;
	}
    }
});
