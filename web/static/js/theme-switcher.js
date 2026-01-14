
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

