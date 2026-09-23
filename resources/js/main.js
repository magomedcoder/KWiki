(function () {
  function openDialog(id) {
    const dialog = document.getElementById(id);
    if (dialog && typeof dialog.showModal === "function" && !dialog.open) {
      dialog.showModal();
    }
  }

  document.querySelectorAll("[data-open-dialog]").forEach(function (button) {
    button.addEventListener("click", function () {
      const menu = button.closest("details");
      if (menu) {
        menu.removeAttribute("open");
      }
      const resetId = button.getAttribute("data-reset-form");
      if (resetId) {
        const resetForm = document.getElementById(resetId);
        if (resetForm) {
          resetForm.reset();
        }
      }
      openDialog(button.getAttribute("data-open-dialog"));
    });
  });

  document.querySelectorAll("[data-close-dialog]").forEach(function (button) {
    button.addEventListener("click", function () {
      const dialog = document.getElementById(button.getAttribute("data-close-dialog"));
      if (dialog) {
        dialog.close();
      }
    });
  });

  document.querySelectorAll("[data-user-edit]").forEach(function (button) {
    button.addEventListener("click", function () {
      const form = document.getElementById("user-edit-form");
      if (!form) {
        return;
      }

      const self = button.getAttribute("data-self") === "1";
      form.elements.original_email.value = button.getAttribute("data-email") || "";
      form.elements.email.value = button.getAttribute("data-email") || "";
      form.elements.first_name.value = button.getAttribute("data-first") || "";
      form.elements.last_name.value = button.getAttribute("data-last") || "";
      form.elements.admin.checked = button.getAttribute("data-admin") === "1";
      if (form.elements.blocked) {
        form.elements.blocked.checked = button.getAttribute("data-blocked") === "1";
      }

      if (form.elements.password) {
        form.elements.password.value = "";
        form.elements.password.disabled = self;
      }

      const blocked = document.getElementById("user-edit-blocked");
      const password = document.getElementById("user-edit-password");
      if (blocked) {
        blocked.hidden = self;
      }

      if (password) {
        password.hidden = self;
      }
      openDialog("user-edit-dialog");
    });
  });

  document.querySelectorAll("[data-user-delete]").forEach(function (button) {
    button.addEventListener("click", function () {
      const form = document.getElementById("user-delete-form");
      const label = document.getElementById("user-delete-label");
      if (form && form.elements.email) {
        form.elements.email.value = button.getAttribute("data-email") || "";
      }

      if (label) {
        label.textContent = "Удалить " + (button.getAttribute("data-name") || button.getAttribute("data-email") || "пользователя") + "?";
      }

      openDialog("user-delete-dialog");
    });
  });

  const reopen = document.querySelector("[data-open-on-load]");
  if (reopen) {
    const reopenId = reopen.getAttribute("data-open-on-load");
    if (reopenId) {
      openDialog(reopenId);
    }
  }

  document.addEventListener("click", function (event) {
    document.querySelectorAll("details.account-menu[open]").forEach(function (menu) {
      if (!menu.contains(event.target)) {
        menu.removeAttribute("open");
      }
    });
  });

  const passwordForm = document.getElementById("password-form");
  if (passwordForm) {
    passwordForm.addEventListener("submit", function (event) {
      event.preventDefault();
      const error = document.getElementById("password-error");
      const submit = passwordForm.querySelector("button[type=submit]");
      if (error) {
        error.hidden = true;
      }
      if (submit) {
        submit.disabled = true;
      }
      fetch("/account/password", {
        method: "POST",
        body: new URLSearchParams(new FormData(passwordForm)),
        credentials: "same-origin"
      }).then(function (response) {
        if (response.status === 204) {
          window.location.assign("/login");
          return;
        }
        return response.text().then(function (text) {
          if (error) {
            error.textContent = (text || "").trim() || "Не удалось сменить пароль";
            error.hidden = false;
          }
        });
      }).catch(function () {
        if (error) {
          error.textContent = "Не удалось сменить пароль";
          error.hidden = false;
        }
      }).finally(function () {
        if (submit) {
          submit.disabled = false;
        }
      });
    });
  }
})();

(function () {
  const form = document.querySelector("form.editor");
  if (!form) {
    return;
  }

  const area = form.querySelector("textarea[name=content]");
  const preview = form.querySelector(".preview-body");
  const split = form.querySelector(".editor-split");
  const csrf = form.querySelector("input[name=csrf]");
  const maxMedia = Number(form.getAttribute("data-max-media") || 2097152);
  let timer;

  function branchName() {
    const el = document.getElementById("editor-branch");
    if (!el) {
      return form.getAttribute("data-branch") || "main";
    }
    return el.value || form.getAttribute("data-branch") || "main";
  }

  function refresh() {
    const body = new URLSearchParams();
    body.set("csrf", csrf.value);
    body.set("branch", branchName());
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

  function insertAtCursor(text) {
    const start = area.selectionStart;
    const end = area.selectionEnd;
    area.setRangeText(text, start, end, "end");
    area.focus();
    refresh();
  }

  function insertMediaMarkdown(ref, alt) {
    insertAtCursor("![" + (alt || "") + "](" + ref + ")");
  }

  area.addEventListener("input", function () {
    clearTimeout(timer);
    timer = setTimeout(refresh, 200);
  });

  form.querySelectorAll(".editor-tools button").forEach(function (button) {
    if (button.hasAttribute("data-media-open")) {
      return;
    }
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
        item.toggleAttribute("data-active", item === button);
      });
      if (mode === "preview") {
        refresh();
      } else {
        area.focus();
      }
    });
  });

  const uploadDialog = document.getElementById("media-upload-dialog");
  const pickDialog = document.getElementById("media-pick-dialog");
  const fileInput = document.getElementById("media-file");
  const filePick = document.getElementById("media-file-pick");
  const fileChosen = document.getElementById("media-file-name");
  const folderPick = document.getElementById("media-folder-pick");
  const folderNew = document.getElementById("media-folder-new");
  const folderInput = document.getElementById("media-folder");
  const nameInput = document.getElementById("media-name");
  const altInput = document.getElementById("media-alt");
  const uploadBtn = document.getElementById("media-upload");
  const uploadError = document.getElementById("media-upload-error");
  const listError = document.getElementById("media-list-error");
  const listEl = document.getElementById("media-list");
  const listEmpty = document.getElementById("media-list-empty");
  const pager = document.getElementById("media-pager");
  const pagePrev = document.getElementById("media-page-prev");
  const pageNext = document.getElementById("media-page-next");
  const pageLabel = document.getElementById("media-page-label");
  const pageSize = 50;
  let mediaItems = [];
  let mediaPage = 0;

  if (!uploadDialog || !pickDialog) {
    return;
  }

  function showMessage(node, message) {
    if (!message) {
      node.classList.add("hidden");
      node.textContent = "";
      return;
    }
    node.textContent = message;
    node.classList.remove("hidden");
  }

  function showUploadError(message) {
    showMessage(uploadError, message);
  }

  function showListError(message) {
    showMessage(listError, message);
  }

  function showDialog(node) {
    if (typeof node.showModal === "function") {
      if (!node.open) {
        node.showModal();
      }
    } else {
      node.setAttribute("open", "");
    }
  }

  function setNameFromFile() {
    if (!fileInput.files || !fileInput.files[0]) {
      fileChosen.textContent = "Файл не выбран";
      return;
    }
    const file = fileInput.files[0];
    fileChosen.textContent = file.name;
    if (!nameInput.value) {
      nameInput.value = file.name;
    }
    if (!altInput.value) {
      altInput.value = file.name;
    }
  }

  filePick.addEventListener("click", function () {
    fileInput.click();
  });
  fileInput.addEventListener("change", setNameFromFile);
  nameInput.addEventListener("input", function () {
    if (!altInput.dataset.touched) {
      altInput.value = nameInput.value;
    }
  });
  altInput.addEventListener("input", function () {
    altInput.dataset.touched = "1";
  });

  function syncNewFolder() {
    const creating = folderPick.value === "__new__";
    folderNew.classList.toggle("hidden", !creating);
    folderNew.classList.toggle("flex", creating);
  }

  function fillFolders(items) {
    const current = folderPick.value;
    const folders = {};
    items.forEach(function (item) {
      const ref = item.Ref || "";
      const slash = ref.lastIndexOf("/");
      if (slash > 0) {
        folders[ref.slice(0, slash)] = true;
      }
    });
    const names = Object.keys(folders).sort();
    folderPick.innerHTML = "";
    const root = document.createElement("option");
    root.value = "";
    root.textContent = "Корень";
    folderPick.appendChild(root);
    names.forEach(function (name) {
      const option = document.createElement("option");
      option.value = name;
      option.textContent = name;
      folderPick.appendChild(option);
    });
    const created = document.createElement("option");
    created.value = "__new__";
    created.textContent = "Новая папка…";
    folderPick.appendChild(created);
    if (current === "__new__" || current === "" || folders[current]) {
      folderPick.value = current;
    }
    syncNewFolder();
  }

  function chosenFolder() {
    if (folderPick.value === "__new__") {
      return (folderInput.value || "").trim();
    }
    return folderPick.value;
  }

  folderPick.addEventListener("change", syncNewFolder);

  pagePrev.addEventListener("click", function () {
    if (mediaPage > 0) {
      mediaPage -= 1;
      renderList();
    }
  });
  pageNext.addEventListener("click", function () {
    mediaPage += 1;
    renderList();
  });
  window.matchMedia("(max-width: 767px)").addEventListener("change", function () {
    if (pickDialog.open) {
      mediaPage = 0;
      renderList();
    }
  });

  function fetchMedia() {
    return fetch("/media?branch=" + encodeURIComponent(branchName()), {
      credentials: "same-origin"
    }).then(function (response) {
      if (!response.ok) {
        throw new Error("list");
      }
      return response.json();
    }).then(function (data) {
      return (data && data.items) || [];
    });
  }

  function listIsMobile() {
    return window.matchMedia("(max-width: 767px)").matches;
  }

  function renderList(items) {
    if (items) {
      mediaItems = items;
    }
    const total = mediaItems.length;
    const paged = listIsMobile() && total > pageSize;
    const pages = paged ? Math.ceil(total / pageSize) : 1;
    if (mediaPage >= pages) {
      mediaPage = pages - 1;
    }
    if (mediaPage < 0) {
      mediaPage = 0;
    }
    const visible = paged ? mediaItems.slice(mediaPage * pageSize, (mediaPage + 1) * pageSize) : mediaItems;
    listEl.innerHTML = "";
    listEmpty.classList.toggle("hidden", total > 0);
    pager.classList.toggle("hidden", !paged);
    pager.classList.toggle("flex", paged);
    if (paged) {
      pageLabel.textContent = (mediaPage + 1) + " / " + pages;
      pagePrev.disabled = mediaPage === 0;
      pageNext.disabled = mediaPage >= pages - 1;
    }
    visible.forEach(function (item) {
      const li = document.createElement("li");
      li.className = "flex flex-wrap items-center justify-between gap-2 border border-wiki-line-soft bg-wiki-well px-2.5 py-2 text-sm";
      const meta = document.createElement("div");
      meta.className = "min-w-0 break-all";
      meta.textContent = item.Path + (item.Staged ? " (черновик)" : "");
      const actions = document.createElement("div");
      actions.className = "flex flex-wrap gap-1.5";
      const insert = document.createElement("button");
      insert.type = "button";
      insert.className = "inline-flex min-h-8 cursor-pointer items-center rounded-sm border border-wiki-line bg-white px-2 py-1";
      insert.textContent = "Вставить";
      insert.addEventListener("click", function () {
        const alt = window.prompt("Подпись (alt)", item.Name || "") || item.Name || "";
        insertMediaMarkdown(item.Ref || item.Name, alt);
        pickDialog.close();
      });
      const remove = document.createElement("button");
      remove.type = "button";
      remove.className = "inline-flex min-h-8 cursor-pointer items-center rounded-sm border border-wiki-danger-line bg-wiki-danger-bg px-2 py-1 text-wiki-danger";
      remove.textContent = "Удалить";
      remove.addEventListener("click", function () {
        if (!window.confirm("Удалить " + item.Path + "?")) {
          return;
        }
        fetch("/media?branch=" + encodeURIComponent(branchName()) + "&path=" + encodeURIComponent(item.Path) + "&csrf=" + encodeURIComponent(csrf.value), {
          method: "DELETE",
          credentials: "same-origin"
        }).then(function (response) {
          if (!response.ok) {
            throw new Error("delete");
          }
          return fetchMedia().then(renderList);
        }).catch(function () {
          showListError("Не удалось удалить файл");
        });
      });
      actions.appendChild(insert);
      actions.appendChild(remove);
      li.appendChild(meta);
      li.appendChild(actions);
      listEl.appendChild(li);
    });
  }

  form.querySelectorAll("[data-media-open]").forEach(function (button) {
    button.addEventListener("click", function () {
      const mode = button.getAttribute("data-media-open");
      if (mode === "upload") {
        showUploadError("");
        altInput.dataset.touched = "";
        showDialog(uploadDialog);
        fetchMedia().then(fillFolders).catch(function () {
          showUploadError("Не удалось загрузить список папок");
        });
        return;
      }
      showListError("");
      mediaPage = 0;
      showDialog(pickDialog);
      fetchMedia().then(renderList).catch(function () {
        showListError("Не удалось загрузить список");
      });
    });
  });

  function upload(overwrite) {
    showUploadError("");
    const file = fileInput.files && fileInput.files[0];
    if (!file) {
      showUploadError("Выберите файл");
      return Promise.resolve();
    }
    if (file.size > maxMedia) {
      showUploadError("Файл больше 2 МБ");
      return Promise.resolve();
    }
    const name = (nameInput.value || file.name || "").trim();
    if (!name) {
      showUploadError("Укажите имя файла");
      return Promise.resolve();
    }

    const body = new FormData();
    body.set("csrf", csrf.value);
    body.set("branch", branchName());
    body.set("folder", chosenFolder());
    body.set("name", name);
    body.set("file", file, name);
    if (overwrite) {
      body.set("overwrite", "1");
    }

    return fetch("/media", {
      method: "POST",
      body: body,
      credentials: "same-origin"
    }).then(function (response) {
      if (response.status === 409) {
        if (window.confirm("Файл уже есть. Перезаписать?")) {
          return upload(true);
        }
        showUploadError("Файл уже есть");
        return null;
      }
      if (response.status === 413) {
        showUploadError("Файл больше 2 МБ");
        return null;
      }
      if (!response.ok) {
        throw new Error("upload");
      }
      return response.json();
    }).then(function (item) {
      if (!item) {
        return;
      }
      const alt = (altInput.value || item.Name || name).trim();
      insertMediaMarkdown(item.Ref || item.Name, alt);
      fileInput.value = "";
      fileChosen.textContent = "Файл не выбран";
      uploadDialog.close();
    });
  }

  uploadBtn.addEventListener("click", function () {
    uploadBtn.disabled = true;
    upload(false).catch(function () {
      showUploadError("Не удалось загрузить файл");
    }).finally(function () {
      uploadBtn.disabled = false;
    });
  });
})();
