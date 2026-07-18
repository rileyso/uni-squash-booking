(() => {
  const sidebar = document.querySelector(".sidebar");
  const toggle = sidebar?.querySelector(".nav-toggle");
  if (!sidebar || !toggle) return;

  const label = toggle.querySelector(".visually-hidden");
  const setExpanded = (expanded, returnFocus = false) => {
    sidebar.classList.toggle("nav-expanded", expanded);
    toggle.setAttribute("aria-expanded", String(expanded));
    if (label) label.textContent = expanded ? "Collapse navigation" : "Expand navigation";
    if (returnFocus) toggle.focus();
  };

  toggle.addEventListener("click", () => {
    setExpanded(toggle.getAttribute("aria-expanded") !== "true");
  });

  sidebar.addEventListener("keydown", (event) => {
    if (event.key === "Escape" && toggle.getAttribute("aria-expanded") === "true") {
      setExpanded(false, true);
    }
  });

  sidebar.querySelectorAll("nav a").forEach((link) => {
    link.addEventListener("click", () => setExpanded(false));
  });

  const desktop = window.matchMedia("(min-width: 881px)");
  desktop.addEventListener("change", (event) => {
    if (event.matches) setExpanded(false);
  });
})();
