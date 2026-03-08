export class VaultBrowser {
  constructor(container) {
    this.container = container;
    this.docs = [];
    this.selectedDoc = null;
    this.searchQuery = "";
    this.selectedTags = new Set();
    this.allTags = [];
    this.tagCounts = new Map();
    this._tagDropdownOpen = false;
    this._expandedDirs = new Set();
    this._build();
  }

  _build() {
    this.container.innerHTML = "";
    this.container.className = "vault-browser";

    // Toolbar (search + tag toggle + stats)
    const toolbar = document.createElement("div");
    toolbar.className = "vault-toolbar";

    const searchWrap = document.createElement("div");
    searchWrap.className = "vault-search-wrap";

    const searchIcon = document.createElement("span");
    searchIcon.className = "vault-search-icon";
    searchIcon.textContent = "\u26B2";

    const searchInput = document.createElement("input");
    searchInput.className = "vault-search";
    searchInput.type = "text";
    searchInput.placeholder = "Search docs...";
    searchInput.spellcheck = false;
    searchInput.addEventListener("input", () => {
      this.searchQuery = searchInput.value;
      if (this.onSearch && this.searchQuery.length >= 2) {
        this.onSearch(this.searchQuery);
      } else if (this.searchQuery.length === 0) {
        this._renderTree();
      }
    });
    this._searchInput = searchInput;

    searchWrap.appendChild(searchIcon);
    searchWrap.appendChild(searchInput);

    const tagToggle = document.createElement("button");
    tagToggle.className = "vault-tag-toggle";
    tagToggle.innerHTML = '<span class="vault-tag-toggle-icon">&#9660;</span> TAGS';
    tagToggle.addEventListener("click", () => this._toggleTagDropdown());
    this._tagToggle = tagToggle;

    const statsEl = document.createElement("span");
    statsEl.className = "vault-stats";
    this._statsEl = statsEl;

    toolbar.appendChild(searchWrap);
    toolbar.appendChild(tagToggle);
    toolbar.appendChild(statsEl);
    this.container.appendChild(toolbar);

    // Active tags strip
    const activeStrip = document.createElement("div");
    activeStrip.className = "vault-active-tags hidden";
    this._activeStrip = activeStrip;
    this.container.appendChild(activeStrip);

    // Tag dropdown
    const tagDropdown = document.createElement("div");
    tagDropdown.className = "vault-tag-dropdown hidden";
    this._tagDropdown = tagDropdown;
    this.container.appendChild(tagDropdown);

    // Content area: tree + detail
    const content = document.createElement("div");
    content.className = "vault-content";

    const tree = document.createElement("div");
    tree.className = "vault-tree";
    this._treeEl = tree;

    const detail = document.createElement("div");
    detail.className = "vault-detail";
    detail.innerHTML = '<div class="vault-detail-empty"><span class="vault-detail-empty-icon">&#9783;</span><br>Select a document</div>';
    this._detailEl = detail;

    content.appendChild(tree);
    content.appendChild(detail);
    this.container.appendChild(content);

    document.addEventListener("click", (e) => {
      if (this._tagDropdownOpen && !tagDropdown.contains(e.target) && !tagToggle.contains(e.target)) {
        this._closeTagDropdown();
      }
    });
  }

  show() { this.container.classList.remove("hidden"); }
  hide() { this.container.classList.add("hidden"); }

  setDocs(docs) {
    this.docs = docs || [];
    const tagSet = new Set();
    const counts = new Map();
    for (const d of this.docs) {
      try {
        const tags = JSON.parse(d.tags || "[]");
        tags.forEach(t => {
          tagSet.add(t);
          counts.set(t, (counts.get(t) || 0) + 1);
        });
      } catch {}
    }
    this.allTags = [...tagSet].sort();
    this.tagCounts = counts;

    // Auto-expand: project-level folders + top-level dirs within each
    if (this._expandedDirs.size === 0) {
      const projects = new Set(this.docs.map(d => d.project || "_unknown"));
      if (projects.size > 1) {
        for (const p of projects) this._expandedDirs.add(`__proj__${p}`);
      }
      const topDirs = new Set();
      for (const d of this.docs) {
        const first = d.path.split("/")[0];
        if (d.path.includes("/")) topDirs.add(first);
      }
      for (const dir of topDirs) this._expandedDirs.add(dir);
    }

    this._renderTagDropdown();
    this._renderActiveStrip();
    this._renderTree();
    this._statsEl.textContent = `${this.docs.length} docs`;
  }

  setSearchResults(results) {
    this._renderSearchResults(results);
  }

  // -- Tag dropdown --

  _toggleTagDropdown() {
    this._tagDropdownOpen ? this._closeTagDropdown() : this._openTagDropdown();
  }

  _openTagDropdown() {
    this._tagDropdownOpen = true;
    this._tagDropdown.classList.remove("hidden");
    this._tagToggle.classList.add("active");
    this._tagToggle.querySelector(".vault-tag-toggle-icon").innerHTML = "&#9650;";
  }

  _closeTagDropdown() {
    this._tagDropdownOpen = false;
    this._tagDropdown.classList.add("hidden");
    this._tagToggle.classList.remove("active");
    this._tagToggle.querySelector(".vault-tag-toggle-icon").innerHTML = "&#9660;";
  }

  _renderTagDropdown() {
    this._tagDropdown.innerHTML = "";
    const grid = document.createElement("div");
    grid.className = "vault-tag-grid";

    for (const tag of this.allTags) {
      const btn = document.createElement("button");
      const isActive = this.selectedTags.has(tag);
      btn.className = "vault-tag-chip" + (isActive ? " active" : "");
      const count = this.tagCounts.get(tag) || 0;
      btn.innerHTML = `<span class="vault-tag-chip-name">${tag}</span><span class="vault-tag-chip-count">${count}</span>`;
      btn.addEventListener("click", (e) => {
        e.stopPropagation();
        if (this.selectedTags.has(tag)) this.selectedTags.delete(tag);
        else this.selectedTags.add(tag);
        this._renderTagDropdown();
        this._renderActiveStrip();
        this._renderTree();
      });
      grid.appendChild(btn);
    }
    this._tagDropdown.appendChild(grid);

    if (this.selectedTags.size > 0) {
      const clearBtn = document.createElement("button");
      clearBtn.className = "vault-tag-clear";
      clearBtn.textContent = "CLEAR ALL";
      clearBtn.addEventListener("click", (e) => {
        e.stopPropagation();
        this.selectedTags.clear();
        this._renderTagDropdown();
        this._renderActiveStrip();
        this._renderTree();
      });
      this._tagDropdown.appendChild(clearBtn);
    }
  }

  _renderActiveStrip() {
    this._activeStrip.innerHTML = "";
    if (this.selectedTags.size === 0) {
      this._activeStrip.classList.add("hidden");
      this._tagToggle.innerHTML = '<span class="vault-tag-toggle-icon">' + (this._tagDropdownOpen ? "&#9650;" : "&#9660;") + '</span> TAGS';
      return;
    }
    this._activeStrip.classList.remove("hidden");
    this._tagToggle.innerHTML = '<span class="vault-tag-toggle-icon">' + (this._tagDropdownOpen ? "&#9650;" : "&#9660;") + '</span> TAGS <span class="vault-tag-badge">' + this.selectedTags.size + '</span>';

    for (const tag of this.selectedTags) {
      const chip = document.createElement("button");
      chip.className = "vault-active-chip";
      chip.innerHTML = `${tag} <span class="vault-active-chip-x">&times;</span>`;
      chip.addEventListener("click", () => {
        this.selectedTags.delete(tag);
        this._renderTagDropdown();
        this._renderActiveStrip();
        this._renderTree();
      });
      this._activeStrip.appendChild(chip);
    }
  }

  // -- File tree --

  _buildTree(docs) {
    const root = { children: new Map(), files: [] };

    for (const doc of docs) {
      const parts = doc.path.split("/");
      const fileName = parts.pop();
      let node = root;

      for (const part of parts) {
        if (!node.children.has(part)) {
          node.children.set(part, { children: new Map(), files: [] });
        }
        node = node.children.get(part);
      }
      node.files.push({ ...doc, fileName });
    }
    return root;
  }

  _renderTree() {
    this._treeEl.innerHTML = "";

    let filtered = this.docs;
    if (this.selectedTags.size > 0) {
      filtered = filtered.filter(d => {
        try {
          const tags = JSON.parse(d.tags || "[]");
          return [...this.selectedTags].every(t => tags.includes(t));
        } catch { return false; }
      });
    }

    if (this.selectedTags.size > 0) {
      this._statsEl.textContent = `${filtered.length} / ${this.docs.length}`;
    } else {
      this._statsEl.textContent = `${this.docs.length} docs`;
    }

    if (filtered.length === 0) {
      this._treeEl.innerHTML = '<div class="vault-empty">No documents match</div>';
      return;
    }

    // Group by project
    const byProject = new Map();
    for (const doc of filtered) {
      const proj = doc.project || "_unknown";
      if (!byProject.has(proj)) byProject.set(proj, []);
      byProject.get(proj).push(doc);
    }

    // Render project-level folders
    const sortedProjects = [...byProject.keys()].sort((a, b) => {
      // _relay always last
      if (a === "_relay") return 1;
      if (b === "_relay") return -1;
      return a.localeCompare(b);
    });

    for (const proj of sortedProjects) {
      const projDocs = byProject.get(proj);
      const projPath = `__proj__${proj}`;
      const isExpanded = this._expandedDirs.has(projPath);

      const dirEl = document.createElement("div");
      dirEl.className = "vt-dir";

      const dirRow = document.createElement("div");
      dirRow.className = "vt-dir-row vt-project-row" + (isExpanded ? " expanded" : "");
      dirRow.style.paddingLeft = "12px";

      const chevron = document.createElement("span");
      chevron.className = "vt-chevron";
      chevron.textContent = isExpanded ? "\u25BE" : "\u25B8";

      const icon = document.createElement("span");
      icon.className = "vt-dir-icon";
      icon.textContent = proj === "_relay" ? "\u2699" : "\uD83D\uDCDA";

      const name = document.createElement("span");
      name.className = "vt-dir-name vt-project-name";
      name.textContent = proj === "_relay" ? "Relay Docs" : proj;

      const count = document.createElement("span");
      count.className = "vt-dir-count";
      count.textContent = projDocs.length;

      dirRow.appendChild(chevron);
      dirRow.appendChild(icon);
      dirRow.appendChild(name);
      dirRow.appendChild(count);

      dirRow.addEventListener("click", () => {
        if (this._expandedDirs.has(projPath)) {
          this._expandedDirs.delete(projPath);
        } else {
          this._expandedDirs.add(projPath);
        }
        this._renderTree();
      });

      dirEl.appendChild(dirRow);

      if (isExpanded) {
        const childContainer = document.createElement("div");
        childContainer.className = "vt-children";
        const tree = this._buildTree(projDocs);
        this._renderNode(tree, childContainer, "", 1);
        dirEl.appendChild(childContainer);
      }

      this._treeEl.appendChild(dirEl);
    }
  }

  _renderNode(node, parentEl, pathPrefix, depth) {
    // Sort: directories first, then files
    const sortedDirs = [...node.children.entries()].sort((a, b) => a[0].localeCompare(b[0]));
    const sortedFiles = node.files.sort((a, b) => (a.fileName).localeCompare(b.fileName));

    for (const [dirName, child] of sortedDirs) {
      const dirPath = pathPrefix ? `${pathPrefix}/${dirName}` : dirName;
      const isExpanded = this._expandedDirs.has(dirPath);
      const fileCount = this._countFiles(child);

      const dirEl = document.createElement("div");
      dirEl.className = "vt-dir";

      const dirRow = document.createElement("div");
      dirRow.className = "vt-dir-row" + (isExpanded ? " expanded" : "");
      dirRow.style.paddingLeft = `${12 + depth * 16}px`;

      const chevron = document.createElement("span");
      chevron.className = "vt-chevron";
      chevron.textContent = isExpanded ? "\u25BE" : "\u25B8";

      const icon = document.createElement("span");
      icon.className = "vt-dir-icon";
      icon.textContent = isExpanded ? "\uD83D\uDCC2" : "\uD83D\uDCC1";

      const name = document.createElement("span");
      name.className = "vt-dir-name";
      name.textContent = dirName;

      const count = document.createElement("span");
      count.className = "vt-dir-count";
      count.textContent = fileCount;

      dirRow.appendChild(chevron);
      dirRow.appendChild(icon);
      dirRow.appendChild(name);
      dirRow.appendChild(count);

      dirRow.addEventListener("click", () => {
        if (this._expandedDirs.has(dirPath)) {
          this._expandedDirs.delete(dirPath);
        } else {
          this._expandedDirs.add(dirPath);
        }
        this._renderTree();
      });

      dirEl.appendChild(dirRow);

      if (isExpanded) {
        const childContainer = document.createElement("div");
        childContainer.className = "vt-children";
        this._renderNode(child, childContainer, dirPath, depth + 1);
        dirEl.appendChild(childContainer);
      }

      parentEl.appendChild(dirEl);
    }

    for (const doc of sortedFiles) {
      const fileEl = document.createElement("div");
      fileEl.className = "vt-file" + (this.selectedDoc === doc.path ? " selected" : "");
      fileEl.style.paddingLeft = `${12 + depth * 16 + 16}px`;

      const icon = document.createElement("span");
      icon.className = "vt-file-icon";
      icon.textContent = this._fileIcon(doc.fileName);

      const name = document.createElement("span");
      name.className = "vt-file-name";
      name.textContent = doc.title || doc.fileName;

      const size = document.createElement("span");
      size.className = "vt-file-size";
      size.textContent = this._formatSize(doc.size_bytes);

      fileEl.appendChild(icon);
      fileEl.appendChild(name);
      fileEl.appendChild(size);

      fileEl.addEventListener("click", () => {
        if (this.onSelectDoc) this.onSelectDoc(doc.path);
      });

      parentEl.appendChild(fileEl);
    }
  }

  _countFiles(node) {
    let count = node.files.length;
    for (const child of node.children.values()) {
      count += this._countFiles(child);
    }
    return count;
  }

  _fileIcon(fileName) {
    if (!fileName) return "\uD83D\uDCC4";
    const ext = fileName.split(".").pop().toLowerCase();
    const icons = {
      md: "\uD83D\uDCDD", txt: "\uD83D\uDCC4", json: "{}",
      yaml: "\u2699", yml: "\u2699", toml: "\u2699",
      go: "\u25C8", js: "\u25C7", ts: "\u25C7", py: "\u25C6",
      sql: "\u25A3", sh: "\u25B7", css: "\u25CB", html: "\u25C9",
    };
    return icons[ext] || "\uD83D\uDCC4";
  }

  _renderSearchResults(results) {
    this._treeEl.innerHTML = "";

    if (!results || results.length === 0) {
      this._treeEl.innerHTML = '<div class="vault-empty">No results</div>';
      return;
    }

    this._statsEl.textContent = `${results.length} results`;

    for (const r of results) {
      const item = document.createElement("div");
      item.className = "vt-search-result";
      item.addEventListener("click", () => {
        if (this.onSelectDoc) this.onSelectDoc(r.path);
      });

      const titleEl = document.createElement("div");
      titleEl.className = "vt-search-title";
      titleEl.textContent = r.title || r.path;

      const pathEl = document.createElement("div");
      pathEl.className = "vt-search-path";
      pathEl.textContent = r.path;

      item.appendChild(titleEl);
      item.appendChild(pathEl);

      if (r.excerpt) {
        const excerptEl = document.createElement("div");
        excerptEl.className = "vt-search-excerpt";
        excerptEl.textContent = (r.excerpt || "").replace(/>>>/g, "").replace(/<<</g, "");
        item.appendChild(excerptEl);
      }

      this._treeEl.appendChild(item);
    }
  }

  // -- Detail view --

  showDocContent(doc) {
    this.selectedDoc = doc.path;
    this._currentDoc = doc;
    this._editing = false;
    this._destroyEditor();
    this._detailEl.innerHTML = "";

    // Header with edit button
    const header = document.createElement("div");
    header.className = "vault-detail-header";

    const titleRow = document.createElement("div");
    titleRow.className = "vault-detail-title-row";

    const titleEl = document.createElement("h2");
    titleEl.className = "vault-detail-title";
    titleEl.textContent = doc.title || doc.path;
    titleRow.appendChild(titleEl);

    const editBtn = document.createElement("button");
    editBtn.className = "vault-edit-btn";
    editBtn.textContent = "EDIT";
    editBtn.addEventListener("click", () => this._enterEditMode());
    titleRow.appendChild(editBtn);

    header.appendChild(titleRow);

    // Breadcrumb path
    const breadcrumb = document.createElement("div");
    breadcrumb.className = "vault-detail-breadcrumb";
    const parts = doc.path.split("/");
    parts.forEach((part, i) => {
      if (i > 0) {
        const sep = document.createElement("span");
        sep.className = "vault-detail-breadcrumb-sep";
        sep.textContent = "/";
        breadcrumb.appendChild(sep);
      }
      const span = document.createElement("span");
      span.className = i === parts.length - 1 ? "vault-detail-breadcrumb-file" : "vault-detail-breadcrumb-dir";
      span.textContent = part;
      breadcrumb.appendChild(span);
    });
    header.appendChild(breadcrumb);

    // Meta
    const metaEl = document.createElement("div");
    metaEl.className = "vault-detail-meta";
    if (doc.owner) {
      const ownerSpan = document.createElement("span");
      ownerSpan.className = "vault-detail-meta-owner";
      ownerSpan.textContent = doc.owner;
      metaEl.appendChild(ownerSpan);
    }
    const sizeSpan = document.createElement("span");
    sizeSpan.textContent = this._formatSize(doc.size_bytes);
    metaEl.appendChild(sizeSpan);
    header.appendChild(metaEl);

    // Tags as clickable pills
    try {
      const tags = JSON.parse(doc.tags || "[]");
      if (tags.length > 0) {
        const tagsRow = document.createElement("div");
        tagsRow.className = "vault-detail-tags-row";
        for (const tag of tags) {
          const pill = document.createElement("button");
          pill.className = "vault-detail-tag-pill" + (this.selectedTags.has(tag) ? " active" : "");
          pill.textContent = tag;
          pill.addEventListener("click", () => {
            this.selectedTags.add(tag);
            this._renderTagDropdown();
            this._renderActiveStrip();
            this._renderTree();
          });
          tagsRow.appendChild(pill);
        }
        header.appendChild(tagsRow);
      }
    } catch {}

    this._detailEl.appendChild(header);

    // Content area (view mode)
    const contentEl = document.createElement("div");
    contentEl.className = "vault-detail-content";
    this._contentEl = contentEl;
    let mdContent = (doc.content || "").replace(/^\s*#\s+.+\n?/, "");
    const rendered = this._renderMarkdown(mdContent);
    if (rendered) {
      contentEl.innerHTML = rendered;
    } else {
      contentEl.innerHTML = '<div class="vault-detail-no-content">No content available</div>';
    }
    this._detailEl.appendChild(contentEl);

    // Editor area (hidden by default)
    const editorArea = document.createElement("div");
    editorArea.className = "vault-editor-area hidden";
    this._editorArea = editorArea;

    const textarea = document.createElement("textarea");
    textarea.id = "vault-easymde";
    editorArea.appendChild(textarea);

    const editorActions = document.createElement("div");
    editorActions.className = "vault-editor-actions";

    const saveBtn = document.createElement("button");
    saveBtn.className = "vault-save-btn";
    saveBtn.textContent = "SAVE";
    saveBtn.addEventListener("click", () => this._saveDoc());

    const cancelBtn = document.createElement("button");
    cancelBtn.className = "vault-cancel-btn";
    cancelBtn.textContent = "CANCEL";
    cancelBtn.addEventListener("click", () => this._exitEditMode());

    const statusEl = document.createElement("span");
    statusEl.className = "vault-save-status";
    this._saveStatus = statusEl;

    editorActions.appendChild(saveBtn);
    editorActions.appendChild(cancelBtn);
    editorActions.appendChild(statusEl);
    editorArea.appendChild(editorActions);

    this._detailEl.appendChild(editorArea);

    // Re-render tree to update selected state
    if (this.searchQuery.length < 2) {
      this._renderTree();
    }
  }

  _enterEditMode() {
    if (this._editing) return;
    this._editing = true;
    this._contentEl.classList.add("hidden");
    this._editorArea.classList.remove("hidden");

    const textarea = this._editorArea.querySelector("textarea");
    this._easyMDE = new EasyMDE({
      element: textarea,
      initialValue: this._currentDoc.content || "",
      spellChecker: false,
      autofocus: true,
      status: false,
      toolbar: ["bold", "italic", "heading", "|", "quote", "unordered-list", "ordered-list", "|", "link", "code", "table", "|", "preview", "side-by-side", "|", "guide"],
      sideBySideFullscreen: false,
      minHeight: "300px",
    });
  }

  _exitEditMode() {
    this._editing = false;
    this._destroyEditor();
    this._editorArea.classList.add("hidden");
    this._contentEl.classList.remove("hidden");
    this._saveStatus.textContent = "";
  }

  _destroyEditor() {
    if (this._easyMDE) {
      this._easyMDE.toTextArea();
      this._easyMDE = null;
    }
  }

  async _saveDoc() {
    if (!this._easyMDE || !this._currentDoc) return;
    const content = this._easyMDE.value();
    this._saveStatus.textContent = "Saving...";
    this._saveStatus.className = "vault-save-status saving";

    const ok = await this.onSaveDoc?.(this._currentDoc.path, content);
    if (ok) {
      this._saveStatus.textContent = "Saved!";
      this._saveStatus.className = "vault-save-status saved";
      this._currentDoc.content = content;
      // Update rendered view
      let mdContent = content.replace(/^\s*#\s+.+\n?/, "");
      const rendered = this._renderMarkdown(mdContent);
      this._contentEl.innerHTML = rendered || '<div class="vault-detail-no-content">No content</div>';
      setTimeout(() => this._exitEditMode(), 800);
    } else {
      this._saveStatus.textContent = "Error!";
      this._saveStatus.className = "vault-save-status error";
    }
  }

  _formatSize(bytes) {
    if (!bytes) return "";
    if (bytes < 1024) return `${bytes}B`;
    if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)}KB`;
    return `${(bytes / (1024 * 1024)).toFixed(1)}MB`;
  }

  _renderMarkdown(text) {
    if (!text) return "";
    if (typeof marked !== "undefined") {
      marked.setOptions({ gfm: true, breaks: true });
      return marked.parse(text);
    }
    // Fallback: plain text with escaped HTML
    return text.replace(/&/g, "&amp;").replace(/</g, "&lt;").replace(/>/g, "&gt;").replace(/\n/g, "<br>");
  }
}
