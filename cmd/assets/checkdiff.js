const reviewPath = (() => {
  if (window.location.pathname.endsWith(".html")) {
    return window.location.pathname.replace(/\.html$/, ".review.json");
  }
  return `${window.location.pathname.replace(/\/$/, "")}/index.review.json`;
})();

async function postJSON(path, payload) {
  const response = await fetch(path, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(payload),
  });
  if (!response.ok) {
    throw new Error(`request failed: ${response.status}`);
  }
}

function setReviewStatus(node, message, type = "") {
  node.textContent = message;
  node.dataset.state = type;
}

function appendSubmittedComment(list, selectedText, commentText) {
  const item = document.createElement("article");
  item.className = "litcode-review-comment";

  const quote = document.createElement("blockquote");
  quote.className = "litcode-review-comment-quote";
  quote.textContent = selectedText;

  const body = document.createElement("p");
  body.className = "litcode-review-comment-body";
  body.textContent = commentText;

  item.append(quote, body);
  list.prepend(item);
}

function extractSelectionContext(scopeText, selectedText) {
  const idx = scopeText.indexOf(selectedText);
  if (idx < 0) {
    return { contextBefore: "", contextAfter: "" };
  }
  return {
    contextBefore: scopeText.slice(Math.max(0, idx - 160), idx).trim(),
    contextAfter: scopeText
      .slice(idx + selectedText.length, idx + selectedText.length + 160)
      .trim(),
  };
}

function createCommentModal(status, comments) {
  const dialog = document.createElement("dialog");
  dialog.className = "litcode-review-modal";

  const form = document.createElement("form");
  form.className = "litcode-review-modal-card";
  form.method = "dialog";

  const title = document.createElement("h3");
  title.className = "litcode-review-modal-title";
  title.textContent = "Comment on selection";

  const meta = document.createElement("p");
  meta.className = "litcode-review-modal-meta";

  const quote = document.createElement("blockquote");
  quote.className = "litcode-review-modal-selection";

  const input = document.createElement("textarea");
  input.className = "litcode-review-modal-input";
  input.placeholder = "Add a review comment";

  const error = document.createElement("p");
  error.className = "litcode-review-modal-error";

  const actions = document.createElement("div");
  actions.className = "litcode-review-modal-actions";

  const cancel = document.createElement("button");
  cancel.type = "button";
  cancel.className = "litcode-review-modal-cancel";
  cancel.textContent = "Cancel";
  cancel.addEventListener("click", () => {
    dialog.close();
  });

  const save = document.createElement("button");
  save.type = "submit";
  save.className = "litcode-review-modal-save";
  save.textContent = "Save comment";

  actions.append(cancel, save);
  form.append(title, meta, quote, input, error, actions);
  dialog.appendChild(form);

  const state = {
    docPath: "",
    selectedText: "",
    contextBefore: "",
    contextAfter: "",
    sourceFile: "",
    sourceStartLine: 0,
    sourceEndLine: 0,
  };

  form.addEventListener("submit", async (event) => {
    event.preventDefault();
    const trimmed = input.value.trim();
    if (trimmed === "") {
      error.textContent = "Enter a comment before saving.";
      input.focus();
      return;
    }

    save.disabled = true;
    cancel.disabled = true;
    error.textContent = "";
    try {
      await postJSON("/__litcode/comment", {
        type: "comment",
        docPath: state.docPath,
        text: state.selectedText,
        contextBefore: state.contextBefore,
        contextAfter: state.contextAfter,
        sourceFile: state.sourceFile,
        sourceStartLine: state.sourceStartLine,
        sourceEndLine: state.sourceEndLine,
        comment: trimmed,
      });
      appendSubmittedComment(comments, state.selectedText, trimmed);
      dialog.close();
      setReviewStatus(status, "Saved comment on selection.", "saved");
    } catch (submitError) {
      error.textContent = `Unable to save comment: ${submitError}`;
    } finally {
      save.disabled = false;
      cancel.disabled = false;
    }
  });

  dialog.addEventListener("close", () => {
    error.textContent = "";
    save.disabled = false;
    cancel.disabled = false;
  });

  return {
    element: dialog,
    open(selectionDetails) {
      state.docPath = selectionDetails.docPath;
      state.selectedText = selectionDetails.selectedText;
      state.contextBefore = selectionDetails.contextBefore ?? "";
      state.contextAfter = selectionDetails.contextAfter ?? "";
      state.sourceFile = selectionDetails.sourceFile ?? "";
      state.sourceStartLine = selectionDetails.sourceStartLine ?? 0;
      state.sourceEndLine = selectionDetails.sourceEndLine ?? 0;

      const sourceLabel =
        state.sourceFile && state.sourceStartLine > 0
          ? `${state.sourceFile} lines ${state.sourceStartLine}-${state.sourceEndLine || state.sourceStartLine}`
          : state.sourceFile;
      meta.textContent = sourceLabel
        ? `${state.docPath} | ${sourceLabel}`
        : state.docPath;
      quote.textContent = state.selectedText;
      input.value = "";
      error.textContent = "";
      dialog.showModal();
      input.focus();
    },
  };
}

function createSelectionButton() {
  const button = document.createElement("button");
  button.type = "button";
  button.hidden = true;
  button.className = "litcode-review-selection-button";
  button.textContent = "Comment";
  return button;
}

function selectionScope(container, article) {
  return (
    container.closest("pre, p, li, blockquote, h1, h2, h3, h4, h5, h6") ||
    article
  );
}

function getSelectionDetails(article, panel, modalElement) {
  const selection = window.getSelection();
  if (!selection || selection.rangeCount === 0 || selection.isCollapsed) {
    return null;
  }

  const selectedText = selection.toString().trim();
  if (selectedText === "") {
    return null;
  }

  const range = selection.getRangeAt(0);
  const container =
    range.commonAncestorContainer.nodeType === Node.TEXT_NODE
      ? range.commonAncestorContainer.parentElement
      : range.commonAncestorContainer;

  if (!container || !article.contains(container)) {
    return null;
  }
  if (panel.contains(container) || modalElement.contains(container)) {
    return null;
  }

  const rect = range.getBoundingClientRect();
  if (rect.width === 0 && rect.height === 0) {
    return null;
  }

  const scope = selectionScope(container, article);
  const { contextBefore, contextAfter } = extractSelectionContext(
    scope.textContent,
    selectedText,
  );
  const sourceBlock = container.closest(".litcode-block");

  return {
    selectedText,
    rect,
    contextBefore,
    contextAfter,
    sourceFile: sourceBlock?.dataset.sourceFile ?? "",
    sourceStartLine: Number(sourceBlock?.dataset.sourceStartLine ?? 0),
    sourceEndLine: Number(sourceBlock?.dataset.sourceEndLine ?? 0),
  };
}

function hideSelectionButton(button) {
  button.hidden = true;
  button.dataset.selectedText = "";
}

function positionSelectionButton(button, rect) {
  button.hidden = false;
  const width = button.offsetWidth || 96;
  const height = button.offsetHeight || 36;
  const left = Math.min(
    window.innerWidth - width - 16,
    Math.max(16, rect.left + rect.width / 2 - width / 2),
  );
  const top = rect.top >= height + 24 ? rect.top - height - 10 : rect.bottom + 10;
  button.style.left = `${left}px`;
  button.style.top = `${Math.max(16, top)}px`;
}

function buildReviewPanel(review) {
  document.body.classList.add("litcode-review-enabled");

  const article = document.querySelector("article");
  if (!article) {
    return;
  }

  const panel = document.createElement("aside");
  panel.className = "litcode-review";

  const header = document.createElement("div");
  header.className = "litcode-review-header";

  const title = document.createElement("h2");
  title.textContent = "Review comments";

  const doc = document.createElement("p");
  doc.className = "litcode-review-doc";
  doc.textContent = review.docPath;

  const status = document.createElement("p");
  status.className = "litcode-review-status";
  setReviewStatus(status, "Highlight text in the document to leave a comment.");

  const summary = document.createElement("textarea");
  summary.className = "litcode-review-summary";
  summary.placeholder = "Optional final summary comment";

  const done = document.createElement("button");
  done.type = "button";
  done.className = "litcode-review-done";
  done.textContent = "Done";

  const comments = document.createElement("div");
  comments.className = "litcode-review-comments";

  const empty = document.createElement("p");
  empty.className = "litcode-review-empty";
  empty.textContent = "No comments yet. Highlight text anywhere in the rendered document to add one.";
  comments.appendChild(empty);

  const commentModal = createCommentModal(status, {
    prepend(node) {
      if (empty.isConnected) {
        empty.remove();
      }
      comments.prepend(node);
    },
  });

  const selectionButton = createSelectionButton();
  let reviewClosed = false;
  let activeSelection = null;

  const openSelectionComment = () => {
    if (reviewClosed || !activeSelection) {
      return;
    }
    commentModal.open({
      docPath: review.docPath,
      ...activeSelection,
    });
    const selection = window.getSelection();
    if (selection) {
      selection.removeAllRanges();
    }
    hideSelectionButton(selectionButton);
  };

  const lockReview = () => {
    reviewClosed = true;
    summary.disabled = true;
    done.disabled = true;
    hideSelectionButton(selectionButton);
  };

  done.addEventListener("click", async () => {
    done.disabled = true;
    setReviewStatus(status, "Submitting final review…", "pending");
    try {
      await postJSON("/__litcode/done", {
        type: "done",
        docPath: review.docPath,
        summary: summary.value.trim(),
      });
      done.textContent = "Done submitted";
      setReviewStatus(status, "Final review submitted. The server will exit.", "saved");
      lockReview();
    } catch (error) {
      done.disabled = false;
      setReviewStatus(status, `Unable to submit final review: ${error}`, "error");
    }
  });

  selectionButton.addEventListener("click", openSelectionComment);

  const updateSelectionButton = () => {
    if (reviewClosed) {
      activeSelection = null;
      hideSelectionButton(selectionButton);
      return;
    }
    const details = getSelectionDetails(article, panel, commentModal.element);
    if (!details) {
      activeSelection = null;
      hideSelectionButton(selectionButton);
      return;
    }
    activeSelection = details;
    positionSelectionButton(selectionButton, details.rect);
  };

  document.addEventListener("selectionchange", updateSelectionButton);
  window.addEventListener("resize", updateSelectionButton);
  window.addEventListener("scroll", updateSelectionButton, { passive: true });

  header.append(title, doc, status, summary, done);
  panel.append(header, comments);

  document.body.append(panel, selectionButton, commentModal.element);
}

async function main() {
  const response = await fetch(reviewPath);
  if (!response.ok) {
    return;
  }
  const review = await response.json();
  if (!review || typeof review.docPath !== "string") {
    return;
  }
  buildReviewPanel(review);
}

main().catch((error) => {
  console.error("checkdiff review panel failed", error);
});
