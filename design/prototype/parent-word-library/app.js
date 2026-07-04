const state = {
  activeType: "all",
  activeStatus: "all",
  editingWordId: null,
  library: {
    id: "main-library",
    words: [
      { id: "w-1", text: "apple", meaningZh: "苹果", phonetic: "/ˈæpl/", wordType: "new", status: "unlearned" },
      { id: "w-2", text: "read", meaningZh: "阅读", phonetic: "/riːd/", wordType: "mistake", status: "reinforce" },
      { id: "w-3", text: "desk", meaningZh: "书桌", phonetic: "/desk/", wordType: "new", status: "learning" },
      { id: "w-4", text: "climb", meaningZh: "攀爬", phonetic: "/klaɪm/", wordType: "mistake", status: "reinforce" },
      { id: "w-5", text: "water", meaningZh: "水", phonetic: "/ˈwɔːtər/", wordType: "new", status: "mastered" },
      { id: "w-6", text: "their", meaningZh: "他们的", phonetic: "/ðer/", wordType: "mistake", status: "reinforce" }
    ]
  }
};

const wordTypeText = {
  new: "新词",
  mistake: "易错词"
};

const wordStatusText = {
  unlearned: "未学",
  learning: "学习中",
  reinforce: "需强化",
  mastered: "已掌握"
};

const $ = (selector) => document.querySelector(selector);

function activeLibrary() {
  return state.library;
}

function uid(prefix) {
  return `${prefix}-${Date.now()}-${Math.floor(Math.random() * 1000)}`;
}

function defaultStatusForType(wordType) {
  return wordType === "mistake" ? "reinforce" : "unlearned";
}

function deleteSavedWord(wordId) {
  const library = activeLibrary();
  const index = library.words.findIndex((word) => word.id === wordId);
  if (index === -1) return;

  const word = library.words[index];
  if (word.status === "unlearned") {
    library.words.splice(index, 1);
    showToast("未学单词已直接删除。");
    return;
  }

  Object.assign(word, {
    deleted: true,
    deletedAt: new Date().toISOString()
  });
  showToast("已从单词库移除，学习记录保留。");
}

function render() {
  renderCategories();
  renderDetail();
}

function renderCategories() {
  const library = activeLibrary();
  const list = $("#categoryList");
  list.innerHTML = "";
  [
    { key: "all", title: "全部单词", desc: "查看新词和易错词", count: library.words.filter((word) => !word.deleted).length },
    { key: "new", title: "新词", desc: "计划学习的单词", count: library.words.filter((word) => word.wordType === "new" && !word.deleted).length },
    { key: "mistake", title: "易错词", desc: "需要重点复习的单词", count: library.words.filter((word) => word.wordType === "mistake" && !word.deleted).length }
  ].forEach((item) => {
    const button = document.createElement("button");
    button.className = `library-card ${item.key === state.activeType ? "active" : ""}`;
    button.type = "button";
    button.innerHTML = `
      <strong>${item.title}</strong>
      <span>${item.desc}</span>
      <span>${item.count} 个单词</span>
    `;
    button.addEventListener("click", () => {
      state.activeType = item.key;
      render();
    });
    list.appendChild(button);
  });
}

function renderDetail() {
  const library = activeLibrary();
  if (!library) return;
  const visibleWords = library.words.filter((word) => !word.deleted);
  $("#totalWords").textContent = visibleWords.length;
  $("#newWords").textContent = visibleWords.filter((word) => word.wordType === "new").length;
  $("#mistakeWords").textContent = visibleWords.filter((word) => word.wordType === "mistake").length;
  renderWords();
}

function renderWords() {
  const library = activeLibrary();
  const query = $("#wordSearch").value.trim().toLowerCase();
  const type = state.activeType;
  const status = state.activeStatus;
  const tbody = $("#wordTable");
  tbody.innerHTML = "";

  const rows = library.words.filter((word) => {
    const matchesQuery = [word.text, word.meaningZh, word.phonetic].some((value) =>
      String(value || "").toLowerCase().includes(query)
    );
    const matchesType = type === "all" || word.wordType === type;
    const matchesStatus = status === "all" || word.status === status;
    return matchesQuery && matchesType && matchesStatus && !word.deleted;
  });

  if (rows.length === 0) {
    tbody.innerHTML = `<tr><td colspan="6">没有匹配的单词。</td></tr>`;
    return;
  }

  rows.forEach((word) => {
    const tr = document.createElement("tr");
    tr.innerHTML = `
      <td><strong>${word.text}</strong></td>
      <td>${word.meaningZh}</td>
      <td>${word.phonetic || "未填写"}</td>
      <td><span class="tag ${word.wordType}">${wordTypeText[word.wordType]}</span></td>
      <td><span class="status-pill ${word.status}">${wordStatusText[word.status] || "未学"}</span></td>
      <td>
        <div class="row-actions">
          <button class="link-btn" type="button" data-action="edit" data-id="${word.id}">编辑</button>
          <button class="link-btn" type="button" data-action="delete" data-id="${word.id}">删除</button>
        </div>
      </td>
    `;
    tbody.appendChild(tr);
  });
}

function showToast(message) {
  const toast = $("#toast");
  toast.textContent = message;
  toast.classList.add("show");
  window.setTimeout(() => toast.classList.remove("show"), 2600);
}

function openModal(html) {
  $("#modalContent").innerHTML = html;
  $("#modalBackdrop").hidden = false;
}

function closeModal() {
  $("#modalBackdrop").hidden = true;
  $("#modalContent").innerHTML = "";
  state.editingWordId = null;
}

function escapeAttr(value) {
  return String(value ?? "")
    .replaceAll("&", "&amp;")
    .replaceAll('"', "&quot;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;");
}

function getExportWords(scope) {
  const selectedScope = scope || $("#exportScope")?.value || "all";
  const library = activeLibrary();
  return library.words.filter((word) => {
    if (word.deleted) return false;
    if (selectedScope === "unlearned") return word.status === "unlearned";
    if (selectedScope === "learning") return word.status === "learning";
    if (selectedScope === "reinforce") return word.status === "reinforce";
    if (selectedScope === "mastered") return word.status === "mastered";
    return true;
  });
}

function renderExportPreview() {
  renderDictationPreview();
}

function renderDictationPreview() {
  const preview = $("#exportPreview");
  if (!preview) return;
  const words = getExportWords();
  if (words.length === 0) {
    preview.innerHTML = `<p class="empty-note">当前范围没有可导出的单词。</p>`;
    return;
  }

  preview.innerHTML = `
    <div class="dictation-sheet">
      <div class="dictation-title">
        <input id="dictationTitle" value="单词默写练习" aria-label="默写表标题" />
        <div>
          <span>姓名：</span><span class="write-line"></span>
          <span>日期：</span><span class="write-line short"></span>
        </div>
      </div>
      <table class="dictation-table">
        <thead>
          <tr>
            <th>序号</th>
            <th>中文</th>
            <th>英文默写</th>
          </tr>
        </thead>
        <tbody>
          ${words.map((word, index) => `
            <tr>
              <td>${index + 1}</td>
              <td><input data-dictation-cell value="${escapeAttr(word.meaningZh)}" /></td>
              <td><div class="answer-lines"></div></td>
            </tr>
          `).join("")}
        </tbody>
      </table>
    </div>
  `;
}

function downloadHtmlFile(filename, html, extension, type) {
  const blob = new Blob([html], { type });
  const url = URL.createObjectURL(blob);
  const link = document.createElement("a");
  link.href = url;
  link.download = filename.endsWith(`.${extension}`) ? filename : `${filename}.${extension}`;
  document.body.appendChild(link);
  link.click();
  link.remove();
  URL.revokeObjectURL(url);
}

function exportDictationWord() {
  const title = $("#dictationTitle")?.value.trim() || "单词默写练习";
  const rows = Array.from(document.querySelectorAll(".dictation-table tbody tr")).map((row) => {
    const cells = row.querySelectorAll("[data-dictation-cell]");
    return {
      index: row.cells[0].textContent.trim(),
      meaning: cells[0]?.value.trim() || ""
    };
  });
  if (rows.length === 0) {
    showToast("没有可导出的默写内容。");
    return;
  }

  const html = `
    <!doctype html>
    <html>
      <head>
        <meta charset="utf-8" />
        <title>${escapeAttr(title)}</title>
        <style>
          body { font-family: "Microsoft YaHei", sans-serif; color: #111; }
          h1 { text-align: center; font-size: 22px; margin: 12px 0 18px; }
          .meta { display: flex; justify-content: space-between; margin-bottom: 14px; font-size: 14px; }
          table { width: 100%; border-collapse: collapse; table-layout: fixed; }
          th, td { border: 1px solid #222; padding: 9px; font-size: 14px; }
          th:nth-child(1), td:nth-child(1) { width: 52px; text-align: center; }
          th:nth-child(2), td:nth-child(2) { width: 34%; }
          td.answer { height: 34px; }
        </style>
      </head>
      <body>
        <h1>${escapeAttr(title)}</h1>
        <div class="meta"><span>姓名：____________</span><span>日期：____________</span></div>
        <table>
          <thead><tr><th>序号</th><th>中文</th><th>英文默写</th></tr></thead>
          <tbody>
            ${rows.map((row) => `<tr><td>${escapeAttr(row.index)}</td><td>${escapeAttr(row.meaning)}</td><td class="answer"></td></tr>`).join("")}
          </tbody>
        </table>
      </body>
    </html>
  `;
  const filename = $("#exportFilename").value.trim() || "单词默写练习";
  downloadHtmlFile(filename, html, "doc", "application/msword;charset=utf-8");
  showToast(`已导出 ${rows.length} 个默写词。`);
}

function openExportModal() {
  openModal(`
    <div class="modal-title">
      <h2 id="modalTitle">导出单词</h2>
      <p>导出为 Word 默写表，只保留中文提示，英文留空给孩子书写。</p>
    </div>
    <div class="export-layout">
      <section class="export-controls">
        <div class="field">
          <label for="exportScope">导出内容</label>
          <select id="exportScope">
            <option value="all">全部单词</option>
            <option value="unlearned">未学</option>
            <option value="learning">学习中</option>
            <option value="reinforce">需强化</option>
            <option value="mastered">已掌握</option>
          </select>
        </div>
        <div class="field">
          <label for="exportFilename">文件名</label>
          <input id="exportFilename" value="单词库" />
        </div>
      </section>
      <section class="export-preview-panel">
        <div class="export-preview-head">
          <h3>导出预览</h3>
        </div>
        <div id="exportPreview"></div>
      </section>
    </div>
    <div class="form-actions">
      <button class="secondary-btn" id="cancelFormBtn" type="button">取消</button>
      <button class="primary-btn" id="downloadExportBtn" type="button">导出 Word</button>
    </div>
  `);
  $("#cancelFormBtn").addEventListener("click", closeModal);
  $("#exportScope").addEventListener("change", renderExportPreview);
  $("#downloadExportBtn").addEventListener("click", exportDictationWord);
  renderExportPreview();
}

function wordForm(word = null) {
  const isEdit = Boolean(word);
  return `
    <div class="modal-title">
      <h2 id="modalTitle">${isEdit ? "编辑单词" : "逐个录入单词"}</h2>
      <p>${isEdit ? "修改已入库单词会保留最近编辑记录。" : "保存后不关闭窗口，方便连续录入。"}</p>
    </div>
    <form id="wordForm">
      <div class="form-grid">
        <div class="field">
          <label for="wordText">${isEdit ? "英文单词（不可修改）" : "英文单词"}</label>
          <input id="wordText" name="text" value="${word?.text || ""}" ${isEdit ? "readonly" : "required"} />
          ${isEdit ? `<span class="field-hint">单词本身不允许修改；如录错，请删除后重新录入。</span>` : ""}
        </div>
        <div class="field">
          <label for="meaningZh">中文</label>
          <input id="meaningZh" name="meaningZh" value="${word?.meaningZh || ""}" required />
        </div>
        <div class="field">
          <label for="phonetic">音标</label>
          <input id="phonetic" name="phonetic" value="${word?.phonetic || ""}" placeholder="/.../" />
        </div>
        <div class="field">
          <label for="wordType">单词类型</label>
          <select id="wordType" name="wordType">
            <option value="new" ${word?.wordType === "new" ? "selected" : ""}>新词</option>
            <option value="mistake" ${word?.wordType === "mistake" ? "selected" : ""}>易错词</option>
          </select>
        </div>
        ${isEdit ? `
          <div class="field">
            <label for="status">状态</label>
            <select id="status" name="status">
              <option value="unlearned" ${word?.status === "unlearned" ? "selected" : ""}>未学</option>
              <option value="learning" ${word?.status === "learning" ? "selected" : ""}>学习中</option>
              <option value="reinforce" ${word?.status === "reinforce" ? "selected" : ""}>需强化</option>
              <option value="mastered" ${word?.status === "mastered" ? "selected" : ""}>已掌握</option>
            </select>
          </div>
        ` : ""}
      </div>
      <div class="form-actions">
        <button class="secondary-btn" type="button" id="cancelFormBtn">取消</button>
        <button class="primary-btn" type="submit">${isEdit ? "保存修改" : "保存并继续"}</button>
      </div>
    </form>
  `;
}

function bindWordForm(word = null) {
  $("#wordForm").addEventListener("submit", (event) => {
    event.preventDefault();
    const data = Object.fromEntries(new FormData(event.currentTarget).entries());
    const library = activeLibrary();

    if (word) {
      if (data.wordType === "mistake" && word.wordType !== "mistake") {
        data.status = "reinforce";
      }
      Object.assign(word, {
        meaningZh: data.meaningZh,
        phonetic: data.phonetic,
        wordType: data.wordType,
        status: data.status,
        lastEditedAt: new Date().toISOString()
      });
      closeModal();
      showToast("单词已更新，孩子学习进度未重置。");
    } else {
      const duplicate = library.words.find((item) => item.text.toLowerCase() === data.text.toLowerCase());
      if (duplicate && !window.confirm(`当前词库已有 ${data.text}，仍然保存吗？`)) return;
      library.words.unshift({
        id: uid("w"),
        text: data.text,
        meaningZh: data.meaningZh,
        phonetic: data.phonetic,
        wordType: data.wordType,
        status: defaultStatusForType(data.wordType)
      });
      event.currentTarget.reset();
      $("#wordType").value = state.activeType === "mistake" ? "mistake" : "new";
      $("#wordText").focus();
      showToast("已保存，继续录入下一个。");
    }
    render();
  });
  $("#cancelFormBtn").addEventListener("click", closeModal);
}

function openManualAdd() {
  openModal(wordForm());
  bindWordForm();
  if (state.activeType === "new" || state.activeType === "mistake") {
    $("#wordType").value = state.activeType;
  }
  $("#wordText").focus();
}

function openEditWord(wordId) {
  const word = activeLibrary().words.find((item) => item.id === wordId);
  openModal(wordForm(word));
  bindWordForm(word);
}

function openPasteImport() {
  openModal(`
    <div class="modal-title">
      <h2 id="modalTitle">粘贴导入</h2>
      <p>每行一个单词，支持“英文 中文 音标”或逗号分隔。确认前只生成草稿。</p>
    </div>
    <div class="field full">
      <label for="pasteText">单词表文本</label>
      <textarea id="pasteText">jump 跳跃 /dʒʌmp/
green 绿色 /ɡriːn/
their 他们的 /ðer/</textarea>
    </div>
    <div class="form-actions">
      <button class="secondary-btn" id="cancelFormBtn" type="button">取消</button>
      <button class="primary-btn" id="parsePasteBtn" type="button">生成草稿预览</button>
    </div>
  `);
  $("#cancelFormBtn").addEventListener("click", closeModal);
  $("#parsePasteBtn").addEventListener("click", () => {
    const text = $("#pasteText").value;
    const rows = text
      .split(/\n+/)
      .map((line) => line.trim())
      .filter(Boolean)
      .map((line, index) => {
        const parts = line.includes(",") ? line.split(",") : line.split(/\s+/);
        return {
          id: uid("draft"),
          rowIndex: index + 1,
          text: parts[0] || "",
          meaningZh: parts[1] || "",
          phonetic: parts[2] || "",
          wordType: "new",
          status: "pending"
        };
      });
    openDraftPreview("粘贴导入草稿", rows);
  });
}

function openPhotoImport() {
  openModal(`
    <div class="modal-title">
      <h2 id="modalTitle">拍照导入</h2>
      <p>图片识别后只生成草稿，家长修正并确认后才会入库。</p>
    </div>
    <div class="draft-grid">
      <div>
        <div class="photo-preview" id="photoPreview">选择或拍摄单词表图片</div>
        <input id="photoInput" type="file" accept="image/*" capture="environment" />
      </div>
      <div>
        <p class="draft-note">原型中使用模拟 OCR 结果，真实实现时这里会展示图片 OCR 的原始文本和低置信度字段。</p>
        <div class="form-actions">
          <button class="secondary-btn" id="cancelFormBtn" type="button">取消</button>
          <button class="primary-btn" id="mockOcrBtn" type="button">生成草稿预览</button>
        </div>
      </div>
    </div>
  `);
  $("#cancelFormBtn").addEventListener("click", closeModal);
  $("#photoInput").addEventListener("change", (event) => {
    const file = event.target.files?.[0];
    if (!file) return;
    const reader = new FileReader();
    reader.onload = () => {
      $("#photoPreview").innerHTML = `<img src="${reader.result}" alt="单词表图片预览" />`;
    };
    reader.readAsDataURL(file);
  });
  $("#mockOcrBtn").addEventListener("click", () => {
    openDraftPreview("拍照识别草稿", [
      { id: uid("draft"), rowIndex: 1, text: "clirnb", meaningZh: "攀爬", phonetic: "/klaɪm/", wordType: "mistake", status: "pending", confidence: 0.62 },
      { id: uid("draft"), rowIndex: 2, text: "window", meaningZh: "窗户", phonetic: "/ˈwɪndoʊ/", wordType: "new", status: "pending", confidence: 0.91 },
      { id: uid("draft"), rowIndex: 3, text: "chair", meaningZh: "椅子", phonetic: "/tʃer/", wordType: "new", status: "pending", confidence: 0.87 }
    ]);
  });
}

function openDraftPreview(title, rows) {
  const rowHtml = rows.map((row) => draftRowHtml(row)).join("");
  openModal(`
    <div class="modal-title">
      <h2 id="modalTitle">${title}</h2>
      <p>请先修正草稿。确认入库前，不会写入正式单词库。</p>
    </div>
    <p class="draft-note">低置信度或重复词需要家长重点检查。示例中可把 clirnb 修正为 climb。</p>
    <div class="table-wrap">
      <table class="draft-table">
        <thead>
          <tr>
            <th>英文单词</th>
            <th>中文</th>
            <th>音标</th>
            <th>类型</th>
            <th>处理</th>
          </tr>
        </thead>
        <tbody id="draftRows">${rowHtml}</tbody>
      </table>
    </div>
    <div class="draft-actions">
      <button class="secondary-btn" id="addDraftRowBtn" type="button">增加一行</button>
      <div>
        <button class="secondary-btn" id="cancelFormBtn" type="button">取消草稿</button>
        <button class="primary-btn" id="confirmDraftBtn" type="button">确认入库</button>
      </div>
    </div>
  `);
  $("#cancelFormBtn").addEventListener("click", closeModal);
  $("#addDraftRowBtn").addEventListener("click", () => {
    $("#draftRows").insertAdjacentHTML("beforeend", draftRowHtml({
      id: uid("draft"),
      text: "",
      meaningZh: "",
      phonetic: "",
      wordType: "new",
      status: "pending"
    }));
  });
  $("#draftRows").addEventListener("click", (event) => {
    const button = event.target.closest("button[data-action='delete-draft']");
    if (!button) return;
    button.closest("tr").remove();
    showToast("草稿行已删除。");
  });
  $("#confirmDraftBtn").addEventListener("click", confirmDraft);
}

function draftRowHtml(row) {
  return `
    <tr data-draft-id="${row.id}">
      <td><input data-field="text" value="${row.text || ""}" /></td>
      <td><input data-field="meaningZh" value="${row.meaningZh || ""}" /></td>
      <td><input data-field="phonetic" value="${row.phonetic || ""}" /></td>
      <td>
        <select data-field="wordType">
          <option value="new" ${row.wordType === "new" ? "selected" : ""}>新词</option>
          <option value="mistake" ${row.wordType === "mistake" ? "selected" : ""}>易错词</option>
        </select>
      </td>
      <td><button class="link-btn" data-action="delete-draft" type="button">删除</button></td>
    </tr>
  `;
}

function confirmDraft() {
  const library = activeLibrary();
  const rows = Array.from(document.querySelectorAll("#draftRows tr"));
  let imported = 0;

  for (const row of rows) {
    const data = {};
    row.querySelectorAll("[data-field]").forEach((input) => {
      data[input.dataset.field] = input.value.trim();
    });
    if (!data.text) {
      row.style.outline = "2px solid #b24b2b";
      showToast("英文单词为空的草稿行不能入库。");
      return;
    }
    if (!data.meaningZh && !window.confirm(`${data.text} 没有中文释义，仍然入库吗？`)) return;
    const duplicate = library.words.find((word) => word.text.toLowerCase() === data.text.toLowerCase() && word.meaningZh === data.meaningZh);
    if (duplicate) {
      continue;
    }
    library.words.unshift({
      id: uid("w"),
      text: data.text,
      meaningZh: data.meaningZh,
      phonetic: data.phonetic,
      wordType: data.wordType,
      status: defaultStatusForType(data.wordType)
    });
    imported += 1;
  }

  closeModal();
  render();
  showToast(`已入库 ${imported} 个单词。`);
}

function bindEvents() {
  $("#wordSearch").addEventListener("input", renderWords);
  $("#wordStatusFilter").addEventListener("change", (event) => {
    state.activeStatus = event.target.value;
    renderWords();
  });
  $("#manualAddBtn").addEventListener("click", openManualAdd);
  $("#pasteImportBtn").addEventListener("click", openPasteImport);
  $("#photoImportBtn").addEventListener("click", openPhotoImport);
  $("#closeModalBtn").addEventListener("click", closeModal);
  $("#modalBackdrop").addEventListener("click", (event) => {
    if (event.target.id === "modalBackdrop") closeModal();
  });
  $("#wordTable").addEventListener("click", (event) => {
    const button = event.target.closest("button[data-action]");
    if (!button) return;
    const word = activeLibrary().words.find((item) => item.id === button.dataset.id);
    if (button.dataset.action === "edit") openEditWord(word.id);
    if (button.dataset.action === "delete") {
      deleteSavedWord(word.id);
      render();
    }
  });
  $("#exportBtn").addEventListener("click", openExportModal);
}

bindEvents();
render();
