(function () {
  const form = document.querySelector("form.editor");
  if (!form) {
    return;
  }

  const area = form.querySelector("textarea[name=content]");
  const preview = form.querySelector(".preview-body");
  const split = form.querySelector(".editor-split");
  const csrf = form.querySelector("input[name=csrf]");
  let timer;

  function refresh() {
    var body = new URLSearchParams();
    body.set("csrf", csrf.value);
    body.set("content", area.value);
    fetch("/edit/preview", {
      method: "POST",
      headers: { "Content-Type": "application/x-www-form-urlencoded" },
      body: body,
      credentials: "same-origin"
    }).then(function (response) {
      if (!response.ok) {
        return;
      }
      return response.text();
    }).then(function (html) {
      if (html) {
        preview.innerHTML = html;
      }
    });
  }

  area.addEventListener("input", function () {
    clearTimeout(timer);
    timer = setTimeout(refresh, 200);
  });

  form.querySelectorAll(".editor-tools button").forEach(function (button) {
    button.addEventListener("click", function () {
      const before = button.getAttribute("data-before") || "";
      const after = button.getAttribute("data-after") || "";
      const placeholder = button.getAttribute("data-placeholder") || "";
      const start = area.selectionStart;
      const end = area.selectionEnd;
      const selected = area.value.slice(start, end) || placeholder;
      area.setRangeText(before + selected + after, start, end, "select");
      area.selectionStart = start + before.length;
      area.selectionEnd = area.selectionStart + selected.length;
      area.focus();
      refresh();
    });
  });

  form.querySelectorAll(".editor-modes button").forEach(function (button) {
    button.addEventListener("click", function () {
      const mode = button.getAttribute("data-mode");
      split.setAttribute("data-mode", mode);
      form.querySelectorAll(".editor-modes button").forEach(function (item) {
        item.classList.toggle("is-active", item === button);
      });
      if (mode === "preview") {
        refresh();
      } else {
        area.focus();
      }
    });
  });
})();
