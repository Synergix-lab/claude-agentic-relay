// kanban.js — Tokyo Neon Night Kanban Board
// 8-bit aesthetic task board for agent-relay dashboard

const STATUS_ORDER = ['pending', 'accepted', 'in-progress', 'done', 'blocked', 'cancelled'];

const VALID_TRANSITIONS = {
  'pending':     ['accepted', 'in-progress', 'cancelled'],
  'accepted':    ['in-progress', 'done', 'cancelled'],
  'in-progress': ['done', 'blocked', 'cancelled'],
  'blocked':     ['in-progress', 'done', 'cancelled'],
  'done':        ['cancelled'],
  'cancelled':   [],
};

const STATUS_COLORS = {
  'pending':     '#ffd93d',
  'accepted':    '#74b9ff',
  'in-progress': '#00e676',
  'done':        '#636e72',
  'blocked':     '#ff6b6b',
  'cancelled':   '#b2bec3',
};

const PRIORITY_COLORS = {
  P0: '#ff6b6b',
  P1: '#ffa502',
  P2: '#a29bfe',
  P3: '#636e72',
};

const STATUS_LABELS = {
  'pending':     'PENDING',
  'accepted':    'ACCEPTED',
  'in-progress': 'IN PROGRESS',
  'done':        'DONE',
  'blocked':     'BLOCKED',
  'cancelled':   'CANCELLED',
};

function timeAgo(dateStr) {
  if (!dateStr) return '';
  const diff = Date.now() - new Date(dateStr).getTime();
  const secs = Math.floor(diff / 1000);
  if (secs < 60) return `${secs}s ago`;
  const mins = Math.floor(secs / 60);
  if (mins < 60) return `${mins}m ago`;
  const hrs = Math.floor(mins / 60);
  if (hrs < 24) return `${hrs}h ago`;
  const days = Math.floor(hrs / 24);
  return `${days}d ago`;
}

function esc(str) {
  if (!str) return '';
  const d = document.createElement('div');
  d.textContent = str;
  return d.innerHTML;
}

const KANBAN_STYLES = `
@import url('https://fonts.googleapis.com/css2?family=JetBrains+Mono:wght@400;600;700&display=swap');

@keyframes p0pulse {
  0%, 100% { box-shadow: 0 0 6px rgba(255,107,107,0.4), 0 0 12px rgba(255,107,107,0.2); border-color: rgba(255,107,107,0.6); }
  50%      { box-shadow: 0 0 14px rgba(255,107,107,0.8), 0 0 28px rgba(255,107,107,0.4); border-color: rgba(255,107,107,1); }
}

@keyframes slideIn {
  from { opacity: 0; transform: translateY(8px); }
  to   { opacity: 1; transform: translateY(0); }
}

@keyframes formIn {
  from { opacity: 0; transform: scale(0.95); }
  to   { opacity: 1; transform: scale(1); }
}

.kb-root {
  font-family: 'JetBrains Mono', monospace;
  background: #0a0a12;
  color: #dfe6e9;
  width: 100%;
  height: 100%;
  display: flex;
  flex-direction: column;
  overflow: hidden;
  position: relative;
}

/* ── Header ── */
.kb-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 14px 20px 10px;
  border-bottom: 1px solid rgba(108,92,231,0.2);
  flex-shrink: 0;
}
.kb-header h2 {
  margin: 0;
  font-size: 15px;
  font-weight: 700;
  letter-spacing: 2px;
  text-transform: uppercase;
  color: #6c5ce7;
  text-shadow: 0 0 10px rgba(108,92,231,0.5);
}
.kb-add-btn {
  width: 32px; height: 32px;
  background: rgba(108,92,231,0.15);
  border: 1px solid rgba(108,92,231,0.35);
  color: #6c5ce7;
  font-size: 20px;
  font-family: 'JetBrains Mono', monospace;
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  transition: all 0.2s;
  line-height: 1;
}
.kb-add-btn:hover {
  background: rgba(108,92,231,0.3);
  box-shadow: 0 0 12px rgba(108,92,231,0.4);
  transform: scale(1.1);
}

/* ── Board tabs ── */
.kb-tab {
  font-family: 'JetBrains Mono', monospace;
  font-size: 9px;
  font-weight: 600;
  padding: 3px 8px;
  background: transparent;
  border: 1px solid rgba(108,92,231,0.15);
  color: #636e72;
  cursor: pointer;
  letter-spacing: 1px;
  transition: all 0.15s;
  border-radius: 2px;
}
.kb-tab:hover {
  border-color: rgba(108,92,231,0.4);
  color: #a29bfe;
}
.kb-tab--active {
  background: rgba(108,92,231,0.2);
  border-color: rgba(108,92,231,0.5);
  color: #6c5ce7;
  text-shadow: 0 0 6px rgba(108,92,231,0.4);
}

/* ── Board ── */
.kb-board {
  display: flex;
  gap: 12px;
  padding: 14px 16px;
  flex: 1;
  overflow-x: auto;
  overflow-y: hidden;
}

/* ── Column ── */
.kb-col {
  flex: 1;
  min-width: 220px;
  max-width: 340px;
  display: flex;
  flex-direction: column;
  background: rgba(15,15,26,0.95);
  border: 1px solid rgba(108,92,231,0.15);
  border-radius: 4px;
  overflow: hidden;
}
.kb-col.kb-col--blocked {
  background: rgba(30,12,12,0.95);
  border-color: rgba(255,107,107,0.2);
}
.kb-col-header {
  padding: 10px 12px 8px;
  text-transform: uppercase;
  font-size: 11px;
  font-weight: 700;
  letter-spacing: 2px;
  display: flex;
  align-items: center;
  gap: 8px;
  border-bottom: 2px solid rgba(108,92,231,0.25);
  flex-shrink: 0;
}
.kb-col--blocked .kb-col-header {
  border-bottom-color: rgba(255,107,107,0.3);
}
.kb-col-count {
  font-size: 10px;
  background: rgba(108,92,231,0.15);
  color: #a29bfe;
  padding: 1px 6px;
  border-radius: 2px;
}
.kb-col-body {
  flex: 1;
  overflow-y: auto;
  padding: 8px;
  display: flex;
  flex-direction: column;
  gap: 8px;
}
.kb-col-body::-webkit-scrollbar { width: 4px; }
.kb-col-body::-webkit-scrollbar-track { background: transparent; }
.kb-col-body::-webkit-scrollbar-thumb { background: rgba(108,92,231,0.3); border-radius: 2px; }
.kb-col.kb-drag-over {
  border-color: rgba(108,92,231,0.6);
  box-shadow: inset 0 0 20px rgba(108,92,231,0.1);
}

/* ── Card ── */
.kb-card {
  background: rgba(30,30,50,0.6);
  border: 1px solid rgba(108,92,231,0.15);
  border-radius: 3px;
  padding: 10px;
  cursor: grab;
  transition: all 0.15s;
  animation: slideIn 0.25s ease-out;
  position: relative;
}
.kb-card:hover {
  border-color: rgba(108,92,231,0.4);
  box-shadow: 0 0 10px rgba(108,92,231,0.15);
  transform: translateY(-1px);
}
.kb-card.kb-dragging {
  opacity: 0.5;
  transform: scale(0.97);
}
.kb-card.kb-p0 {
  border-color: rgba(255,107,107,0.6);
  animation: p0pulse 2s ease-in-out infinite, slideIn 0.25s ease-out;
}
.kb-card.kb-founder {
  border-left: 3px solid #ffd93d;
  background: rgba(255,217,61,0.06);
}
.kb-card.kb-highlight {
  animation: kb-flash 0.6s ease-in-out 3;
  border-color: rgba(108,92,231,0.8);
  box-shadow: 0 0 16px rgba(108,92,231,0.3), 0 0 4px rgba(108,92,231,0.2);
}
@keyframes kb-flash {
  0%, 100% { box-shadow: 0 0 16px rgba(108,92,231,0.3); }
  50% { box-shadow: 0 0 24px rgba(108,92,231,0.6), 0 0 8px rgba(108,92,231,0.4); border-color: rgba(108,92,231,1); }
}
.kb-founder-tag {
  font-size: 8px;
  font-weight: 700;
  color: #ffd93d;
  background: rgba(255,217,61,0.15);
  border: 1px solid rgba(255,217,61,0.3);
  padding: 0 4px;
  border-radius: 2px;
  letter-spacing: 1px;
  text-transform: uppercase;
}
.kb-card-top {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 6px;
}
.kb-badge {
  font-size: 9px;
  font-weight: 700;
  padding: 1px 5px;
  border-radius: 2px;
  letter-spacing: 1px;
}
.kb-time {
  font-size: 9px;
  color: #636e72;
}
.kb-card-title {
  font-size: 12px;
  font-weight: 600;
  color: #dfe6e9;
  margin-bottom: 6px;
  line-height: 1.35;
  word-break: break-word;
}
.kb-card-meta {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  align-items: center;
  font-size: 10px;
}
.kb-profile {
  color: #a29bfe;
}
.kb-agent {
  color: #00e676;
}
.kb-card-actions {
  position: absolute;
  bottom: 6px;
  right: 6px;
  display: none;
}
.kb-card:hover .kb-card-actions { display: flex; gap: 4px; }
.kb-action-btn {
  width: 18px; height: 18px;
  background: rgba(108,92,231,0.2);
  border: 1px solid rgba(108,92,231,0.3);
  color: #a29bfe;
  font-size: 10px;
  cursor: pointer;
  display: flex;
  align-items: center;
  justify-content: center;
  font-family: 'JetBrains Mono', monospace;
  transition: all 0.15s;
  padding: 0;
  line-height: 1;
}
.kb-action-btn:hover {
  background: rgba(108,92,231,0.4);
  color: #fff;
}
.kb-action-btn.kb-action-block {
  border-color: rgba(255,107,107,0.3);
  color: #ff6b6b;
}
.kb-action-btn.kb-action-block:hover {
  background: rgba(255,107,107,0.3);
}

/* ── Card detail ── */
.kb-detail {
  margin-top: 8px;
  padding-top: 8px;
  border-top: 1px solid rgba(108,92,231,0.15);
  font-size: 10px;
  color: #b2bec3;
  animation: slideIn 0.2s ease-out;
}
.kb-detail-row {
  margin-bottom: 4px;
  line-height: 1.4;
}
.kb-detail-label {
  color: #636e72;
  text-transform: uppercase;
  font-size: 9px;
  letter-spacing: 1px;
}
.kb-detail-desc {
  white-space: pre-wrap;
  word-break: break-word;
  margin: 4px 0;
  padding: 6px;
  background: rgba(10,10,18,0.5);
  border-radius: 2px;
  max-height: 120px;
  overflow-y: auto;
}
.kb-subtask-list {
  padding-left: 12px;
  margin: 4px 0;
}
.kb-subtask-item {
  margin-bottom: 2px;
}
.kb-subtask-status {
  font-size: 9px;
  padding: 0 3px;
  border-radius: 2px;
  margin-left: 4px;
}

/* ── Dispatch form ── */
.kb-overlay {
  position: absolute;
  inset: 0;
  background: rgba(5,5,10,0.85);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 100;
}
.kb-form {
  background: rgba(15,15,26,0.98);
  border: 1px solid rgba(108,92,231,0.3);
  border-radius: 4px;
  padding: 24px;
  width: 560px;
  max-width: 90%;
  max-height: 85vh;
  overflow-y: auto;
  animation: formIn 0.2s ease-out;
  box-shadow: 0 0 30px rgba(108,92,231,0.15);
}
.kb-form::-webkit-scrollbar { width: 4px; }
.kb-form::-webkit-scrollbar-track { background: transparent; }
.kb-form::-webkit-scrollbar-thumb { background: rgba(108,92,231,0.3); border-radius: 2px; }
.kb-form h3 {
  margin: 0 0 16px;
  font-size: 13px;
  color: #6c5ce7;
  text-transform: uppercase;
  letter-spacing: 2px;
  text-shadow: 0 0 8px rgba(108,92,231,0.4);
}
.kb-field {
  margin-bottom: 12px;
}
.kb-field label {
  display: block;
  font-size: 10px;
  color: #636e72;
  text-transform: uppercase;
  letter-spacing: 1px;
  margin-bottom: 4px;
}
.kb-field input,
.kb-field textarea,
.kb-field select {
  width: 100%;
  background: rgba(30,30,50,0.6);
  border: 1px solid rgba(108,92,231,0.2);
  color: #dfe6e9;
  font-family: 'JetBrains Mono', monospace;
  font-size: 12px;
  padding: 7px 10px;
  border-radius: 2px;
  outline: none;
  transition: border-color 0.15s;
  box-sizing: border-box;
}
.kb-field input:focus,
.kb-field textarea:focus,
.kb-field select:focus {
  border-color: rgba(108,92,231,0.6);
  box-shadow: 0 0 8px rgba(108,92,231,0.2);
}
.kb-field textarea {
  resize: vertical;
  min-height: 200px;
}

/* ── Checklist ── */
.kb-checklist {
  margin-top: 4px;
}
.kb-checklist-item {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 5px 8px;
  border-radius: 2px;
  transition: background 0.1s;
}
.kb-checklist-item:hover {
  background: rgba(108,92,231,0.08);
}
.kb-checklist-item input[type="checkbox"] {
  width: 14px;
  height: 14px;
  accent-color: #6c5ce7;
  cursor: pointer;
  flex-shrink: 0;
}
.kb-checklist-item input[type="text"] {
  flex: 1;
  background: transparent;
  border: none;
  color: #dfe6e9;
  font-family: 'JetBrains Mono', monospace;
  font-size: 11px;
  padding: 2px 4px;
  outline: none;
}
.kb-checklist-item input[type="text"]:focus {
  border-bottom: 1px solid rgba(108,92,231,0.4);
}
.kb-checklist-item.kb-checked input[type="text"] {
  text-decoration: line-through;
  color: #636e72;
}
.kb-checklist-remove {
  background: none;
  border: none;
  color: #636e72;
  font-size: 14px;
  cursor: pointer;
  padding: 0 4px;
  line-height: 1;
  opacity: 0;
  transition: opacity 0.15s, color 0.15s;
}
.kb-checklist-item:hover .kb-checklist-remove {
  opacity: 1;
}
.kb-checklist-remove:hover {
  color: #ff6b6b;
}
.kb-checklist-add {
  display: flex;
  align-items: center;
  gap: 6px;
  margin-top: 6px;
  padding: 4px 8px;
  background: none;
  border: 1px dashed rgba(108,92,231,0.2);
  border-radius: 2px;
  color: #636e72;
  font-family: 'JetBrains Mono', monospace;
  font-size: 10px;
  cursor: pointer;
  transition: all 0.15s;
  width: 100%;
}
.kb-checklist-add:hover {
  border-color: rgba(108,92,231,0.5);
  color: #a29bfe;
  background: rgba(108,92,231,0.05);
}
.kb-checklist-progress {
  font-size: 9px;
  color: #636e72;
  margin-top: 4px;
  display: flex;
  align-items: center;
  gap: 6px;
}
.kb-checklist-bar {
  flex: 1;
  height: 3px;
  background: rgba(108,92,231,0.15);
  border-radius: 2px;
  overflow: hidden;
}
.kb-checklist-bar-fill {
  height: 100%;
  background: #6c5ce7;
  border-radius: 2px;
  transition: width 0.2s;
}
.kb-form-btns {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
  margin-top: 16px;
}
.kb-form-btn {
  font-family: 'JetBrains Mono', monospace;
  font-size: 11px;
  font-weight: 600;
  padding: 6px 16px;
  border-radius: 2px;
  cursor: pointer;
  letter-spacing: 1px;
  text-transform: uppercase;
  transition: all 0.15s;
  border: 1px solid;
}
.kb-form-btn--cancel {
  background: transparent;
  border-color: rgba(108,92,231,0.2);
  color: #636e72;
}
.kb-form-btn--cancel:hover {
  border-color: rgba(108,92,231,0.4);
  color: #a29bfe;
}
.kb-form-btn--submit {
  background: rgba(108,92,231,0.2);
  border-color: rgba(108,92,231,0.5);
  color: #6c5ce7;
}
.kb-form-btn--submit:hover {
  background: rgba(108,92,231,0.35);
  box-shadow: 0 0 12px rgba(108,92,231,0.3);
}

/* ── Context menu ── */
.kb-ctx-menu {
  position: fixed;
  background: rgba(15,15,26,0.98);
  border: 1px solid rgba(108,92,231,0.3);
  border-radius: 3px;
  padding: 4px 0;
  z-index: 200;
  min-width: 140px;
  box-shadow: 0 4px 20px rgba(0,0,0,0.6);
  animation: slideIn 0.12s ease-out;
}
.kb-ctx-item {
  padding: 6px 14px;
  font-size: 11px;
  font-family: 'JetBrains Mono', monospace;
  color: #dfe6e9;
  cursor: pointer;
  transition: background 0.1s;
}
.kb-ctx-item:hover {
  background: rgba(108,92,231,0.15);
}
.kb-ctx-item--danger {
  color: #ff6b6b;
}
.kb-ctx-item--danger:hover {
  background: rgba(255,107,107,0.15);
}

/* ── Empty state ── */
.kb-empty {
  flex: 1;
  display: flex;
  align-items: center;
  justify-content: center;
  color: #636e72;
  font-size: 12px;
  letter-spacing: 1px;
  text-transform: uppercase;
}
.kb-col-empty {
  flex: 1;
  display: flex;
  align-items: center;
  justify-content: center;
  color: #444;
  font-size: 10px;
  letter-spacing: 1px;
  text-transform: uppercase;
  padding: 20px 0;
}

/* ── Print styles ── */
@media print {
  .kb-root {
    background: #fff !important;
    color: #111 !important;
    overflow: visible !important;
    height: auto !important;
  }
  .kb-header {
    border-bottom: 2px solid #333 !important;
  }
  .kb-header h2 {
    color: #111 !important;
    text-shadow: none !important;
  }
  .kb-add-btn {
    display: none !important;
  }
  .kb-board {
    overflow: visible !important;
    flex-wrap: nowrap !important;
    gap: 6px !important;
    padding: 10px 8px !important;
  }
  .kb-col {
    background: #fafafa !important;
    border: 1.5px solid #ccc !important;
    min-width: 0 !important;
    max-width: none !important;
    flex: 1 1 0 !important;
    break-inside: avoid;
    page-break-inside: avoid;
  }
  .kb-col--blocked {
    background: #fff5f5 !important;
    border-color: #e88 !important;
  }
  .kb-col-header {
    border-bottom: 1.5px solid #999 !important;
    padding: 6px 8px !important;
    font-size: 10px !important;
  }
  .kb-col-count {
    background: #eee !important;
    color: #333 !important;
  }
  .kb-col-body {
    overflow: visible !important;
    padding: 4px !important;
    gap: 4px !important;
  }
  .kb-card {
    background: #fff !important;
    border: 1px solid #bbb !important;
    padding: 6px 8px !important;
    animation: none !important;
    box-shadow: none !important;
    cursor: default !important;
  }
  .kb-card.kb-p0 {
    border: 2px solid #c00 !important;
    animation: none !important;
  }
  .kb-card-title {
    color: #111 !important;
    font-size: 11px !important;
  }
  .kb-badge {
    border: 1px solid #999 !important;
    font-size: 8px !important;
  }
  .kb-card-actions {
    display: none !important;
  }
  .kb-profile { color: #555 !important; }
  .kb-agent { color: #060 !important; }
  .kb-time { color: #888 !important; }
  .kb-overlay { display: none !important; }
  .kb-ctx-menu { display: none !important; }
}
`;

export class KanbanBoard {
  constructor(container) {
    this.container = container;
    this.tasks = [];
    this.boards = [];
    this.goals = [];
    this.goalMap = new Map(); // id -> goal
    this.selectedBoard = null; // null = all tasks
    this.selectedGoal = null;  // null = all goals
    this.showDone = false;
    this.expandedCard = null;
    this.dragTaskId = null;

    /** @type {((taskId: string, newStatus: string, agentName?: string) => void)|null} */
    this.onTransition = null;
    /** @type {((data: {profile: string, title: string, description: string, priority: string, parent_task_id?: string}) => void)|null} */
    this.onDispatch = null;
    /** @type {((taskId: string, project: string) => void)|null} */
    this.onDelete = null;
    /** @type {((taskId: string, project: string, data: {title?: string, description?: string, priority?: string}) => void)|null} */
    this.onEdit = null;

    // Inject styles
    this._style = document.createElement('style');
    this._style.textContent = KANBAN_STYLES;
    document.head.appendChild(this._style);

    // Root element
    this.root = document.createElement('div');
    this.root.className = 'kb-root';
    this.container.appendChild(this.root);

    // Close context menu on any click
    this._onDocClick = () => this._closeCtxMenu();
    document.addEventListener('click', this._onDocClick);

    this._ctxMenu = null;
    this._overlay = null;
    this._timeInterval = setInterval(() => this._updateTimes(), 30000);

    this._render();
  }

  /* ─── Public API ─── */

  setTasks(tasks) {
    this.tasks = tasks || [];
    // Don't re-render while user is interacting with a form
    if (this._overlay) return;
    this._render();
  }

  setBoards(boards) {
    this.boards = boards || [];
    if (this._overlay) return;
    this._render();
  }

  setGoals(goals) {
    this.goals = goals || [];
    this.goalMap.clear();
    for (const g of this.goals) {
      this.goalMap.set(g.id, g);
    }
    if (this._overlay) return;
    this._render();
  }

  show() {
    this.root.style.display = 'flex';
  }

  hide() {
    this.root.style.display = 'none';
  }

  highlightTask(taskId) {
    const card = this.root.querySelector(`[data-task-id="${taskId}"]`);
    if (!card) return;
    card.scrollIntoView({ behavior: 'smooth', block: 'center' });
    card.classList.add('kb-highlight');
    setTimeout(() => card.classList.remove('kb-highlight'), 2500);
  }

  destroy() {
    clearInterval(this._timeInterval);
    document.removeEventListener('click', this._onDocClick);
    this._closeCtxMenu();
    if (this._style.parentNode) this._style.parentNode.removeChild(this._style);
    if (this.root.parentNode) this.root.parentNode.removeChild(this.root);
  }

  /* ─── Rendering ─── */

  _render() {
    // Save scroll positions of columns before nuke
    const scrollPositions = {};
    this.root.querySelectorAll('.kb-col-body').forEach(body => {
      const status = body.parentElement?.dataset?.status;
      if (status && body.scrollTop > 0) scrollPositions[status] = body.scrollTop;
    });
    // Save root scroll position
    const rootScroll = this.root.scrollTop;

    this.root.innerHTML = '';

    // Header
    const header = document.createElement('div');
    header.className = 'kb-header';

    const titleArea = document.createElement('div');
    titleArea.style.cssText = 'display:flex;align-items:center;gap:12px';
    titleArea.innerHTML = `<h2>Task Board</h2>`;

    // Board tabs
    if (this.boards.length > 0) {
      const tabs = document.createElement('div');
      tabs.style.cssText = 'display:flex;gap:4px;align-items:center';

      const allTab = document.createElement('button');
      allTab.className = 'kb-tab' + (this.selectedBoard === null ? ' kb-tab--active' : '');
      allTab.textContent = 'ALL';
      allTab.addEventListener('click', () => { this.selectedBoard = null; this._render(); });
      tabs.appendChild(allTab);

      for (const b of this.boards.filter(b => !b.archived_at)) {
        const tab = document.createElement('button');
        tab.className = 'kb-tab' + (this.selectedBoard === b.id ? ' kb-tab--active' : '');
        tab.textContent = b.name.toUpperCase();
        tab.title = b.description || b.slug;
        tab.addEventListener('click', () => { this.selectedBoard = b.id; this._render(); });
        tabs.appendChild(tab);
      }
      titleArea.appendChild(tabs);
    }


    header.appendChild(titleArea);

    // Controls: hide-done checkbox + add button
    const controls = document.createElement('div');
    controls.style.cssText = 'display:flex;align-items:center;gap:10px';

    const doneLabel = document.createElement('label');
    doneLabel.style.cssText = 'display:flex;align-items:center;gap:4px;font-size:10px;color:#636e72;cursor:pointer;letter-spacing:1px;text-transform:uppercase';
    const doneCheck = document.createElement('input');
    doneCheck.type = 'checkbox';
    doneCheck.checked = this.showDone;
    doneCheck.style.cssText = 'accent-color:#6c5ce7;cursor:pointer';
    doneCheck.addEventListener('change', () => { this.showDone = doneCheck.checked; this._render(); });
    doneLabel.appendChild(doneCheck);
    doneLabel.appendChild(document.createTextNode('Done'));
    controls.appendChild(doneLabel);

    const addBtn = document.createElement('button');
    addBtn.className = 'kb-add-btn';
    addBtn.textContent = '+';
    addBtn.title = 'Dispatch new task';
    addBtn.addEventListener('click', () => this._showDispatchForm());
    controls.appendChild(addBtn);

    header.appendChild(controls);
    this.root.appendChild(header);

    // Filter tasks
    let filtered = this.tasks;
    if (this.selectedBoard !== null) {
      filtered = filtered.filter(t => t.board_id === this.selectedBoard);
    }
    if (this.selectedGoal !== null) {
      filtered = filtered.filter(t => t.goal_id === this.selectedGoal);
    }
    if (!this.showDone) {
      filtered = filtered.filter(t => t.status !== 'done');
    }

    // Columns to show
    const visibleStatuses = this.showDone ? STATUS_ORDER : STATUS_ORDER.filter(s => s !== 'done');

    // Group tasks by status
    const groups = {};
    for (const t of filtered) {
      const s = t.status || 'pending';
      if (!groups[s]) groups[s] = [];
      groups[s].push(t);
    }

    const board = document.createElement('div');
    board.className = 'kb-board';

    for (const status of visibleStatuses) {
      board.appendChild(this._renderColumn(status, groups[status] || []));
    }

    this.root.appendChild(board);

    // Restore scroll positions
    this.root.scrollTop = rootScroll;
    this.root.querySelectorAll('.kb-col-body').forEach(body => {
      const status = body.parentElement?.dataset?.status;
      if (status && scrollPositions[status]) body.scrollTop = scrollPositions[status];
    });
  }

  _renderColumn(status, tasks) {
    const col = document.createElement('div');
    col.className = 'kb-col' + (status === 'blocked' ? ' kb-col--blocked' : '');
    col.dataset.status = status;

    // Header
    const hdr = document.createElement('div');
    hdr.className = 'kb-col-header';
    hdr.style.color = STATUS_COLORS[status] || '#dfe6e9';
    hdr.innerHTML = `
      <span>${STATUS_LABELS[status] || status.toUpperCase()}</span>
      <span class="kb-col-count">${tasks.length}</span>
    `;
    col.appendChild(hdr);

    // Body
    const body = document.createElement('div');
    body.className = 'kb-col-body';

    if (tasks.length === 0) {
      const empty = document.createElement('div');
      empty.className = 'kb-col-empty';
      empty.textContent = '—';
      body.appendChild(empty);
    } else {
      // Sort: P0 first, then P1, P2, P3
      const prioOrder = { P0: 0, P1: 1, P2: 2, P3: 3 };
      tasks.sort((a, b) => (prioOrder[a.priority] ?? 9) - (prioOrder[b.priority] ?? 9));

      for (const task of tasks) {
        body.appendChild(this._renderCard(task));
      }
    }

    // Drag-and-drop zone (user is admin — allow any move)
    body.addEventListener('dragover', (e) => {
      if (!this.dragTaskId) return;
      e.preventDefault();
      e.dataTransfer.dropEffect = 'move';
      col.classList.add('kb-drag-over');
    });
    body.addEventListener('dragleave', () => {
      col.classList.remove('kb-drag-over');
    });
    body.addEventListener('drop', (e) => {
      e.preventDefault();
      col.classList.remove('kb-drag-over');
      const taskId = e.dataTransfer.getData('text/plain');
      if (taskId && this.dragTaskId === taskId) {
        const task = this.tasks.find(t => t.id === taskId);
        if (task && task.status !== status) {
          if (this.onTransition) {
            this.onTransition(taskId, status, task.assigned_to || null);
          }
        }
      }
    });

    col.appendChild(body);
    return col;
  }

  _renderCard(task) {
    const isFounder = task.profile_slug === 'founder' || task.profile_slug === 'user' || task.profile_slug === 'human';
    const card = document.createElement('div');
    card.className = 'kb-card' + (task.priority === 'P0' ? ' kb-p0' : '') + (isFounder ? ' kb-founder' : '');
    card.draggable = true;
    card.dataset.taskId = task.id;

    // Drag events
    card.addEventListener('dragstart', (e) => {
      this.dragTaskId = task.id;
      card.classList.add('kb-dragging');
      e.dataTransfer.setData('text/plain', task.id);
      e.dataTransfer.effectAllowed = 'move';
    });
    card.addEventListener('dragend', () => {
      card.classList.remove('kb-dragging');
      this.dragTaskId = null;
      // Remove drag-over from all columns
      this.root.querySelectorAll('.kb-drag-over').forEach(el => el.classList.remove('kb-drag-over'));
    });

    // Priority badge
    const prioColor = PRIORITY_COLORS[task.priority] || PRIORITY_COLORS.P2;
    const badgeBg = prioColor + '25';

    // Top row
    const top = document.createElement('div');
    top.className = 'kb-card-top';
    top.innerHTML = `
      <span class="kb-badge" style="color:${prioColor};background:${badgeBg};border:1px solid ${prioColor}40">${esc(task.priority || 'P2')}</span>
      <span class="kb-time" data-dispatched="${esc(task.dispatched_at)}">${timeAgo(task.dispatched_at)}</span>
    `;
    card.appendChild(top);

    // Goal badge
    if (task.goal_id && this.goalMap.has(task.goal_id)) {
      const goal = this.goalMap.get(task.goal_id);
      const goalBadge = document.createElement('div');
      goalBadge.style.cssText = 'font-size:9px;font-weight:600;color:#ffd93d;margin-bottom:3px;letter-spacing:0.5px;white-space:nowrap;overflow:hidden;text-overflow:ellipsis';
      goalBadge.textContent = goal.title;
      goalBadge.title = `[${goal.type}] ${goal.title}`;
      card.appendChild(goalBadge);
    }

    // Title
    const title = document.createElement('div');
    title.className = 'kb-card-title';
    title.textContent = task.title || '(untitled)';
    card.appendChild(title);

    // Meta
    const meta = document.createElement('div');
    meta.className = 'kb-card-meta';
    if (isFounder) {
      meta.innerHTML += `<span class="kb-founder-tag">FOUNDER</span>`;
    } else if (task.profile_slug) {
      meta.innerHTML += `<span class="kb-profile">${esc(task.profile_slug)}</span>`;
    }
    if (task.assigned_to) {
      meta.innerHTML += `<span class="kb-agent">${esc(task.assigned_to)}</span>`;
    }
    card.appendChild(meta);

    // Action buttons (visible on hover)
    const actions = document.createElement('div');
    actions.className = 'kb-card-actions';

    if (task.status !== 'done') {
      const doneBtn = document.createElement('button');
      doneBtn.className = 'kb-action-btn';
      doneBtn.textContent = '\u2713';
      doneBtn.title = 'Mark done';
      doneBtn.addEventListener('click', (e) => {
        e.stopPropagation();
        if (this.onTransition) this.onTransition(task.id, 'done', task.assigned_to || null);
      });
      actions.appendChild(doneBtn);
    }

    if (task.status !== 'blocked' && task.status !== 'done') {
      const blockBtn = document.createElement('button');
      blockBtn.className = 'kb-action-btn kb-action-block';
      blockBtn.textContent = '\u2715';
      blockBtn.title = 'Block';
      blockBtn.addEventListener('click', (e) => {
        e.stopPropagation();
        if (this.onTransition) this.onTransition(task.id, 'blocked', task.assigned_to || null);
      });
      actions.appendChild(blockBtn);
    }
    card.appendChild(actions);

    // Context menu
    card.addEventListener('contextmenu', (e) => {
      e.preventDefault();
      e.stopPropagation();
      this._showCtxMenu(e.clientX, e.clientY, task);
    });

    // Click to expand detail
    card.addEventListener('click', (e) => {
      if (e.target.closest('.kb-action-btn')) return;
      this._toggleDetail(card, task);
    });

    // If this card is expanded, show detail
    if (this.expandedCard === task.id) {
      card.appendChild(this._buildDetail(task));
    }

    return card;
  }

  /* ─── Card Detail ─── */

  _toggleDetail(card, task) {
    if (this.expandedCard === task.id) {
      this.expandedCard = null;
      const detail = card.querySelector('.kb-detail');
      if (detail) detail.remove();
    } else {
      // Collapse any existing expanded card
      const prev = this.root.querySelector('.kb-detail');
      if (prev) prev.remove();
      this.expandedCard = task.id;
      card.appendChild(this._buildDetail(task));
    }
  }

  _buildDetail(task) {
    const detail = document.createElement('div');
    detail.className = 'kb-detail';

    let html = '';

    // Goal ancestry
    if (task.goal_id && this.goalMap.has(task.goal_id)) {
      const chain = this._buildGoalChain(task.goal_id);
      if (chain.length > 0) {
        html += `<div class="kb-detail-row"><span class="kb-detail-label">Goal Cascade</span><div style="margin:3px 0;color:#ffd93d;font-size:10px">${chain.map(g => esc(g.title)).join(' &rsaquo; ')}</div></div>`;
      }
    }

    if (task.description) {
      html += `<div class="kb-detail-row"><span class="kb-detail-label">Description</span><div class="kb-detail-desc">${esc(task.description)}</div></div>`;
    }

    if (task.result) {
      html += `<div class="kb-detail-row"><span class="kb-detail-label">Result</span><div class="kb-detail-desc">${esc(task.result)}</div></div>`;
    }

    if (task.blocked_reason) {
      html += `<div class="kb-detail-row"><span class="kb-detail-label">Blocked Reason</span><div class="kb-detail-desc" style="border-left:2px solid #ff6b6b;padding-left:8px">${esc(task.blocked_reason)}</div></div>`;
    }

    if (task.dispatched_by) {
      html += `<div class="kb-detail-row"><span class="kb-detail-label">Dispatched by</span> <span style="color:#a29bfe">${esc(task.dispatched_by)}</span></div>`;
    }

    if (task.parent_task_id) {
      html += `<div class="kb-detail-row"><span class="kb-detail-label">Parent task</span> <span style="color:#636e72">${esc(task.parent_task_id)}</span></div>`;
    }

    // Timestamps
    const timestamps = [];
    if (task.dispatched_at) timestamps.push(['Dispatched', task.dispatched_at]);
    if (task.accepted_at) timestamps.push(['Accepted', task.accepted_at]);
    if (task.started_at) timestamps.push(['Started', task.started_at]);
    if (task.completed_at) timestamps.push(['Completed', task.completed_at]);
    if (timestamps.length) {
      html += `<div class="kb-detail-row" style="margin-top:6px"><span class="kb-detail-label">Timeline</span>`;
      for (const [label, ts] of timestamps) {
        const d = new Date(ts);
        const formatted = d.toLocaleString(undefined, { month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit' });
        html += `<div style="margin-left:4px;color:#636e72">${label}: <span style="color:#b2bec3">${formatted}</span></div>`;
      }
      html += `</div>`;
    }

    // Subtasks
    if (task.subtasks && task.subtasks.length > 0) {
      html += `<div class="kb-detail-row" style="margin-top:6px"><span class="kb-detail-label">Subtasks (${task.subtasks.length})</span><div class="kb-subtask-list">`;
      for (const sub of task.subtasks) {
        const stColor = STATUS_COLORS[sub.status] || '#636e72';
        html += `<div class="kb-subtask-item">${esc(sub.title || sub.id)} <span class="kb-subtask-status" style="color:${stColor};background:${stColor}20">${esc(sub.status)}</span></div>`;
      }
      html += `</div></div>`;
    }

    detail.innerHTML = html;
    return detail;
  }

  /* ─── Context Menu ─── */

  _showCtxMenu(x, y, task) {
    this._closeCtxMenu();

    const menu = document.createElement('div');
    menu.className = 'kb-ctx-menu';
    menu.style.left = x + 'px';
    menu.style.top = y + 'px';

    const statuses = STATUS_ORDER.filter(s => s !== task.status);

    for (const s of statuses) {
      const item = document.createElement('div');
      item.className = 'kb-ctx-item' + (s === 'blocked' ? ' kb-ctx-item--danger' : '');
      item.innerHTML = `<span style="color:${STATUS_COLORS[s]}">&#9654;</span> ${STATUS_LABELS[s]}`;
      item.addEventListener('click', (e) => {
        e.stopPropagation();
        this._closeCtxMenu();
        if (this.onTransition) this.onTransition(task.id, s, task.assigned_to || null);
      });
      menu.appendChild(item);
    }

    // Separator
    const sep = document.createElement('div');
    sep.style.cssText = 'height:1px;background:rgba(108,92,231,0.2);margin:4px 0';
    menu.appendChild(sep);

    // Edit
    const editItem = document.createElement('div');
    editItem.className = 'kb-ctx-item';
    editItem.innerHTML = `<span style="color:#a29bfe">&#9998;</span> EDIT`;
    editItem.addEventListener('click', (e) => {
      e.stopPropagation();
      this._closeCtxMenu();
      this._showEditForm(task);
    });
    menu.appendChild(editItem);

    // Delete
    const delItem = document.createElement('div');
    delItem.className = 'kb-ctx-item kb-ctx-item--danger';
    delItem.innerHTML = `<span style="color:#ff6b6b">&#10006;</span> DELETE`;
    delItem.addEventListener('click', (e) => {
      e.stopPropagation();
      this._closeCtxMenu();
      if (this.onDelete) this.onDelete(task.id, task.project || 'default');
    });
    menu.appendChild(delItem);

    document.body.appendChild(menu);
    this._ctxMenu = menu;

    // Clamp to viewport
    requestAnimationFrame(() => {
      const rect = menu.getBoundingClientRect();
      if (rect.right > window.innerWidth) menu.style.left = (window.innerWidth - rect.width - 8) + 'px';
      if (rect.bottom > window.innerHeight) menu.style.top = (window.innerHeight - rect.height - 8) + 'px';
    });
  }

  _closeCtxMenu() {
    if (this._ctxMenu) {
      this._ctxMenu.remove();
      this._ctxMenu = null;
    }
  }

  /* ─── Checklist helpers ─── */

  _parseChecklist(description) {
    if (!description) return { text: '', items: [] };
    const lines = description.split('\n');
    const textLines = [];
    const items = [];
    for (const line of lines) {
      const match = line.match(/^- \[([ xX])\] (.*)$/);
      if (match) {
        items.push({ checked: match[1] !== ' ', text: match[2] });
      } else {
        textLines.push(line);
      }
    }
    while (textLines.length && textLines[textLines.length - 1].trim() === '') textLines.pop();
    return { text: textLines.join('\n'), items };
  }

  _buildChecklistHTML(items) {
    const total = items.length;
    const done = items.filter(i => i.checked).length;
    let html = '<div class="kb-checklist">';
    if (total > 0) {
      const pct = Math.round((done / total) * 100);
      html += `<div class="kb-checklist-progress"><span>${done}/${total}</span><div class="kb-checklist-bar"><div class="kb-checklist-bar-fill" style="width:${pct}%"></div></div></div>`;
    }
    items.forEach((item, i) => {
      html += `<div class="kb-checklist-item${item.checked ? ' kb-checked' : ''}" data-idx="${i}">
        <input type="checkbox" ${item.checked ? 'checked' : ''} />
        <input type="text" value="${esc(item.text)}" />
        <button class="kb-checklist-remove" title="Remove">&times;</button>
      </div>`;
    });
    html += `<button class="kb-checklist-add" type="button">+ Add item</button>`;
    html += '</div>';
    return html;
  }

  _attachChecklistEvents(container, items, onUpdate) {
    container.querySelectorAll('.kb-checklist-item input[type="checkbox"]').forEach(cb => {
      cb.addEventListener('change', () => {
        const idx = parseInt(cb.closest('.kb-checklist-item').dataset.idx);
        items[idx].checked = cb.checked;
        cb.closest('.kb-checklist-item').classList.toggle('kb-checked', cb.checked);
        onUpdate();
      });
    });
    container.querySelectorAll('.kb-checklist-item input[type="text"]').forEach(input => {
      input.addEventListener('input', () => {
        const idx = parseInt(input.closest('.kb-checklist-item').dataset.idx);
        items[idx].text = input.value;
      });
    });
    container.querySelectorAll('.kb-checklist-remove').forEach(btn => {
      btn.addEventListener('click', () => {
        const idx = parseInt(btn.closest('.kb-checklist-item').dataset.idx);
        items.splice(idx, 1);
        this._refreshChecklist(container, items, onUpdate);
      });
    });
    const addBtn = container.querySelector('.kb-checklist-add');
    if (addBtn) {
      addBtn.addEventListener('click', () => {
        items.push({ checked: false, text: '' });
        this._refreshChecklist(container, items, onUpdate);
        requestAnimationFrame(() => {
          const newItems = container.querySelectorAll('.kb-checklist-item input[type="text"]');
          if (newItems.length) newItems[newItems.length - 1].focus();
        });
      });
    }
  }

  _refreshChecklist(container, items, onUpdate) {
    container.innerHTML = this._buildChecklistHTML(items);
    this._attachChecklistEvents(container, items, onUpdate);
    onUpdate();
  }

  _serializeDescription(text, items) {
    let desc = text.trim();
    if (items.length > 0) {
      if (desc) desc += '\n\n';
      desc += items.map(i => `- [${i.checked ? 'x' : ' '}] ${i.text}`).join('\n');
    }
    return desc;
  }

  /* ─── Dispatch Form ─── */

  _showDispatchForm() {
    if (this._overlay) return;

    const overlay = document.createElement('div');
    overlay.className = 'kb-overlay';

    const form = document.createElement('div');
    form.className = 'kb-form';
    form.innerHTML = `
      <h3>Dispatch Task</h3>
      <div class="kb-field">
        <label>Profile</label>
        <input type="text" name="profile" placeholder="profile-slug" autocomplete="off" />
      </div>
      <div class="kb-field">
        <label>Title</label>
        <input type="text" name="title" placeholder="Task title" autocomplete="off" />
      </div>
      <div class="kb-field">
        <label>Description</label>
        <textarea name="description" placeholder="Task description..." rows="6"></textarea>
      </div>
      <div class="kb-field">
        <label>Checklist</label>
        <div class="kb-checklist-container"></div>
      </div>
      <div class="kb-field">
        <label>Priority</label>
        <select name="priority">
          <option value="P0">P0 - Critical</option>
          <option value="P1">P1 - High</option>
          <option value="P2" selected>P2 - Normal</option>
          <option value="P3">P3 - Low</option>
        </select>
      </div>
      <div class="kb-field">
        <label>Parent Task ID (optional)</label>
        <input type="text" name="parent_task_id" placeholder="parent-task-uuid" autocomplete="off" />
      </div>
      <div class="kb-field">
        <label>Goal (optional)</label>
        <select name="goal_id">
          <option value="">— None —</option>
          ${this.goals.map(g => `<option value="${esc(g.id)}">[${esc(g.type)}] ${esc(g.title)}</option>`).join('')}
        </select>
      </div>
      <div class="kb-form-btns">
        <button class="kb-form-btn kb-form-btn--cancel" type="button">Cancel</button>
        <button class="kb-form-btn kb-form-btn--submit" type="button">Dispatch</button>
      </div>
    `;

    // Init checklist
    const dispatchChecklistItems = [];
    const dispatchChecklistContainer = form.querySelector('.kb-checklist-container');
    const updateDispatchChecklist = () => {
      const total = dispatchChecklistItems.length;
      const done = dispatchChecklistItems.filter(i => i.checked).length;
      const progress = dispatchChecklistContainer.querySelector('.kb-checklist-progress');
      if (progress) {
        progress.querySelector('span').textContent = `${done}/${total}`;
        progress.querySelector('.kb-checklist-bar-fill').style.width = total ? `${Math.round((done / total) * 100)}%` : '0%';
      }
    };
    dispatchChecklistContainer.innerHTML = this._buildChecklistHTML(dispatchChecklistItems);
    this._attachChecklistEvents(dispatchChecklistContainer, dispatchChecklistItems, updateDispatchChecklist);

    // Cancel
    form.querySelector('.kb-form-btn--cancel').addEventListener('click', () => this._closeDispatchForm());

    // Submit
    form.querySelector('.kb-form-btn--submit').addEventListener('click', () => {
      const profile = form.querySelector('[name="profile"]').value.trim();
      const title = form.querySelector('[name="title"]').value.trim();
      const rawDesc = form.querySelector('[name="description"]').value.trim();
      const description = this._serializeDescription(rawDesc, dispatchChecklistItems);
      const priority = form.querySelector('[name="priority"]').value;
      const parentId = form.querySelector('[name="parent_task_id"]').value.trim();
      const goalId = form.querySelector('[name="goal_id"]').value;

      if (!profile || !title) return;

      const data = { profile, title, description, priority };
      if (parentId) data.parent_task_id = parentId;
      if (goalId) data.goal_id = goalId;

      if (this.onDispatch) this.onDispatch(data);
      this._closeDispatchForm();
    });

    // Close on overlay click
    overlay.addEventListener('click', (e) => {
      if (e.target === overlay) this._closeDispatchForm();
    });

    // Close on Escape
    this._formEscHandler = (e) => {
      if (e.key === 'Escape') this._closeDispatchForm();
    };
    document.addEventListener('keydown', this._formEscHandler);

    overlay.appendChild(form);
    this.root.appendChild(overlay);
    this._overlay = overlay;

    // Focus first input
    requestAnimationFrame(() => form.querySelector('input').focus());
  }

  _closeDispatchForm() {
    if (this._overlay) {
      this._overlay.remove();
      this._overlay = null;
    }
    if (this._formEscHandler) {
      document.removeEventListener('keydown', this._formEscHandler);
      this._formEscHandler = null;
    }
    // Re-render with latest data that may have arrived while form was open
    this._render();
  }

  /* ─── Edit Form ─── */

  _showEditForm(task) {
    if (this._overlay) return;

    const overlay = document.createElement('div');
    overlay.className = 'kb-overlay';

    const form = document.createElement('div');
    form.className = 'kb-form';

    // Parse existing checklist items from description
    const parsed = this._parseChecklist(task.description || '');
    const editChecklistItems = [...parsed.items];

    form.innerHTML = `
      <h3>Edit Task</h3>
      <div class="kb-field">
        <label>Title</label>
        <input type="text" name="title" value="${esc(task.title)}" autocomplete="off" />
      </div>
      <div class="kb-field">
        <label>Description</label>
        <textarea name="description" rows="6">${esc(parsed.text)}</textarea>
      </div>
      <div class="kb-field">
        <label>Checklist</label>
        <div class="kb-checklist-container"></div>
      </div>
      <div class="kb-field">
        <label>Priority</label>
        <select name="priority">
          <option value="P0"${task.priority === 'P0' ? ' selected' : ''}>P0 - Critical</option>
          <option value="P1"${task.priority === 'P1' ? ' selected' : ''}>P1 - High</option>
          <option value="P2"${task.priority === 'P2' ? ' selected' : ''}>P2 - Normal</option>
          <option value="P3"${task.priority === 'P3' ? ' selected' : ''}>P3 - Low</option>
        </select>
      </div>
      <div class="kb-form-btns">
        <button class="kb-form-btn kb-form-btn--cancel" type="button">Cancel</button>
        <button class="kb-form-btn kb-form-btn--submit" type="button">Save</button>
      </div>
    `;

    // Init checklist
    const editChecklistContainer = form.querySelector('.kb-checklist-container');
    const updateEditChecklist = () => {
      const total = editChecklistItems.length;
      const done = editChecklistItems.filter(i => i.checked).length;
      const progress = editChecklistContainer.querySelector('.kb-checklist-progress');
      if (progress) {
        progress.querySelector('span').textContent = `${done}/${total}`;
        progress.querySelector('.kb-checklist-bar-fill').style.width = total ? `${Math.round((done / total) * 100)}%` : '0%';
      }
    };
    editChecklistContainer.innerHTML = this._buildChecklistHTML(editChecklistItems);
    this._attachChecklistEvents(editChecklistContainer, editChecklistItems, updateEditChecklist);

    form.querySelector('.kb-form-btn--cancel').addEventListener('click', () => this._closeDispatchForm());
    form.querySelector('.kb-form-btn--submit').addEventListener('click', () => {
      const title = form.querySelector('[name="title"]').value.trim();
      const rawDesc = form.querySelector('[name="description"]').value.trim();
      const description = this._serializeDescription(rawDesc, editChecklistItems);
      const priority = form.querySelector('[name="priority"]').value;
      if (!title) return;
      if (this.onEdit) this.onEdit(task.id, task.project || 'default', { title, description, priority });
      this._closeDispatchForm();
    });

    overlay.addEventListener('click', (e) => {
      if (e.target === overlay) this._closeDispatchForm();
    });
    this._formEscHandler = (e) => {
      if (e.key === 'Escape') this._closeDispatchForm();
    };
    document.addEventListener('keydown', this._formEscHandler);

    overlay.appendChild(form);
    this.root.appendChild(overlay);
    this._overlay = overlay;
    requestAnimationFrame(() => form.querySelector('input').focus());
  }

  /* ─── Time updater ─── */

  _updateTimes() {
    this.root.querySelectorAll('.kb-time[data-dispatched]').forEach(el => {
      el.textContent = timeAgo(el.dataset.dispatched);
    });
  }

  _buildGoalChain(goalId) {
    const chain = [];
    let current = goalId;
    const visited = new Set();
    while (current && !visited.has(current) && chain.length < 5) {
      visited.add(current);
      const g = this.goalMap.get(current);
      if (!g) break;
      chain.unshift(g);
      current = g.parent_goal_id;
    }
    return chain;
  }
}
