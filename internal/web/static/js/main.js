import { CanvasEngine } from "./canvas.js";
import { World } from "./world.js";
import { AgentView } from "./agent-view.js";
import { APIClient } from "./api-client.js";
import { MessageOrb } from "./message-orb.js";

// DOM elements
const canvas = document.getElementById("relay-canvas");
const statusDot = document.getElementById("status-dot");
const agentCountEl = document.getElementById("agent-count");
const projectSelect = document.getElementById("project-select");
const convSelect = document.getElementById("conv-select");
const messagesTitle = document.getElementById("messages-title");
const messagesList = document.getElementById("messages-list");
const detailPanel = document.getElementById("agent-detail");
const detailName = document.getElementById("detail-name");
const detailRole = document.getElementById("detail-role");
const detailDesc = document.getElementById("detail-desc");
const detailStatus = document.getElementById("detail-status");
const detailLastSeen = document.getElementById("detail-last-seen");
const detailRegistered = document.getElementById("detail-registered");
const detailClose = document.getElementById("detail-close");
const detailReportsTo = document.getElementById("detail-reports-to");
const detailDirectReports = document.getElementById("detail-direct-reports");
const userQuestionsPanel = document.getElementById("user-questions");

// State
const engine = new CanvasEngine(canvas);
const world = new World();
const agentViews = new Map(); // name → AgentView
let conversations = [];       // cached conversation list
let selectedConvId = null;    // currently selected conversation
let paletteCounter = 0;
let agentsData = [];          // cached raw agent data for hierarchy

engine.add(world);
engine.start();

// Mark as connected once we get first data
let connected = false;

// --- Agent layout: arc of circle ---

function layoutAgents() {
  const count = agentViews.size;
  if (count === 0) return;

  const cx = engine.width / 2;
  const cy = engine.height / 2;
  const radius = Math.min(engine.width, engine.height) * 0.3;

  let i = 0;
  for (const [, av] of agentViews) {
    const angle = -Math.PI / 2 + (i / count) * Math.PI * 2;
    av.targetX = cx + Math.cos(angle) * radius;
    av.targetY = cy + Math.sin(angle) * radius;
    i++;
  }
}

// --- API callbacks ---

function onAgents(agents) {
  console.log("[relay] onAgents:", agents.length, "agents");
  if (!connected) {
    connected = true;
    statusDot.classList.add("connected");
  }

  agentCountEl.textContent = `${agents.length} agent${agents.length !== 1 ? "s" : ""}`;

  const currentNames = new Set(agents.map(a => a.name));

  // Remove agents that no longer exist
  for (const [name, av] of agentViews) {
    if (!currentNames.has(name)) {
      engine.remove(av);
      agentViews.delete(name);
    }
  }

  // Add/update agents
  for (const a of agents) {
    let av = agentViews.get(a.name);
    if (!av) {
      av = new AgentView(a.name, a.role, a.description, paletteCounter++, a.online);
      av.setPosition(engine.width / 2, engine.height / 2);
      av.spawnEffect();
      agentViews.set(a.name, av);
      engine.add(av);
    } else {
      av.online = a.online;
      av.role = a.role;
      av.description = a.description;
    }
    av._reportsTo = a.reports_to || null;
  }

  agentsData = agents;
  layoutAgents();
  updateHighlights();
  updateHierarchyLinks();
}

function onConversations(convs) {
  console.log("[relay] onConversations:", convs.length, "conversations");
  conversations = convs;

  // Rebuild dropdown (preserving selection)
  const prev = convSelect.value;
  convSelect.innerHTML = '<option value="">-- All conversations --</option>';

  for (const c of convs) {
    const opt = document.createElement("option");
    opt.value = c.id;
    const memberCount = (c.members || []).length;
    const msgCount = c.message_count || 0;
    opt.textContent = `${c.title} (${memberCount} members, ${msgCount} msgs)`;
    convSelect.appendChild(opt);
  }

  // Restore previous selection if still valid
  if (prev && convs.some(c => c.id === prev)) {
    convSelect.value = prev;
  } else if (!prev && !selectedConvId) {
    // Default: show all conversations
    convSelect.value = "";
    selectedConvId = null;
    updateHighlights();
    loadConversationMessages();
  }
}

function onNewMessages(msgs) {
  // Check for user_question messages to display in the UI
  checkForUserQuestions(msgs);

  for (const msg of msgs) {
    // Animate orb between sender and receiver
    const fromAv = agentViews.get(msg.from);
    const toAv = msg.to === "*" ? null : agentViews.get(msg.to);

    if (fromAv) {
      // Show bubble on sender
      const preview = msg.subject || msg.content.slice(0, 80);
      fromAv.showBubble(preview, "speech");
    }

    if (fromAv && toAv) {
      // Direct message: orb from sender to receiver
      const orb = new MessageOrb(
        fromAv.x, fromAv.y,
        toAv.x, toAv.y,
        msg.type || "default",
        () => engine.remove(orb)
      );
      engine.add(orb);
    } else if (fromAv && msg.to === "*") {
      // Broadcast: orbs from sender to all others
      for (const [name, av] of agentViews) {
        if (name !== msg.from) {
          const orb = new MessageOrb(
            fromAv.x, fromAv.y,
            av.x, av.y,
            msg.type || "notification",
            () => engine.remove(orb)
          );
          engine.add(orb);
        }
      }
    } else if (fromAv && msg.conversation_id) {
      // Conversation message: orbs to all conversation members
      const conv = conversations.find(c => c.id === msg.conversation_id);
      if (conv && conv.members) {
        for (const member of conv.members) {
          if (member !== msg.from) {
            const targetAv = agentViews.get(member);
            if (targetAv) {
              const orb = new MessageOrb(
                fromAv.x, fromAv.y,
                targetAv.x, targetAv.y,
                msg.type || "default",
                () => engine.remove(orb)
              );
              engine.add(orb);
            }
          }
        }
      }
    }

    // If this message belongs to the selected conversation, append to panel
    if (selectedConvId && msg.conversation_id === selectedConvId) {
      appendMessage(msg);
    }
  }
}

// --- Project selection ---

async function loadProjects() {
  const projects = await client.fetchProjects();
  projectSelect.innerHTML = "";

  if (projects.length === 0) {
    const opt = document.createElement("option");
    opt.value = "default";
    opt.textContent = "default";
    projectSelect.appendChild(opt);
  } else {
    for (const p of projects) {
      const opt = document.createElement("option");
      opt.value = p;
      opt.textContent = p;
      projectSelect.appendChild(opt);
    }
  }

  // Select first project
  if (projectSelect.options.length > 0) {
    projectSelect.value = projectSelect.options[0].value;
    client.setProject(projectSelect.value);
  }

  // Update world display
  world.projectName = client.project;
}

projectSelect.addEventListener("change", () => {
  client.setProject(projectSelect.value);
  world.projectName = client.project;

  // Clear all agent views
  for (const [, av] of agentViews) {
    engine.remove(av);
  }
  agentViews.clear();
  paletteCounter = 0;

  // Clear conversations
  conversations = [];
  selectedConvId = null;
  convSelect.innerHTML = '<option value="">-- All conversations --</option>';

  // Re-fetch for the new project
  client.fetchAgents();
  client.fetchConversations();
  loadConversationMessages();
});

// --- Conversation selection ---

convSelect.addEventListener("change", async () => {
  selectedConvId = convSelect.value || null;
  updateHighlights();
  await loadConversationMessages();
});

function updateHighlights() {
  if (!selectedConvId) {
    // No conversation selected → show all agents normally
    for (const [, av] of agentViews) {
      av.highlighted = true;
      av.dimMode = false;
    }
    return;
  }

  const conv = conversations.find(c => c.id === selectedConvId);
  const members = new Set(conv?.members || []);

  for (const [name, av] of agentViews) {
    const isMember = members.has(name);
    av.highlighted = isMember;
    av.dimMode = true; // Enable dimming mode (non-highlighted agents appear dimmed)
  }
}

async function loadConversationMessages() {
  messagesList.innerHTML = "";

  if (!selectedConvId) {
    // Show ALL messages from all conversations
    messagesTitle.textContent = "All Messages";

    const msgs = await client.fetchAllMessages();

    if (msgs.length === 0) {
      messagesList.innerHTML = '<div class="msg-empty">No messages yet</div>';
      return;
    }

    for (const msg of msgs) {
      appendMessage(msg, true);
    }

    messagesList.scrollTop = messagesList.scrollHeight;
    return;
  }

  const conv = conversations.find(c => c.id === selectedConvId);
  messagesTitle.textContent = conv ? conv.title : "Messages";

  const msgs = await client.fetchConversationMessages(selectedConvId);

  if (msgs.length === 0) {
    messagesList.innerHTML = '<div class="msg-empty">No messages yet</div>';
    return;
  }

  for (const msg of msgs) {
    appendMessage(msg);
  }

  messagesList.scrollTop = messagesList.scrollHeight;
}

function appendMessage(msg, showConv = false) {
  const el = document.createElement("div");
  el.className = "msg-item";

  const time = formatTime(msg.created_at);
  const subject = msg.subject ? `<span class="msg-subject">${escapeHtml(msg.subject)}</span>` : "";
  const content = msg.content.length > 500 ? msg.content.slice(0, 497) + "..." : msg.content;

  // Show conversation tag when viewing all messages
  let convTag = "";
  if (showConv && msg.conversation_id) {
    const conv = conversations.find(c => c.id === msg.conversation_id);
    const convName = conv ? conv.title : "DM";
    convTag = `<span class="msg-conv-tag">${escapeHtml(convName)}</span> `;
  }

  el.innerHTML = `
    ${subject}
    ${convTag}<span class="msg-from">${escapeHtml(msg.from)}</span>
    <span class="msg-content">${escapeHtml(content)}</span>
    <div class="msg-time">${time}</div>
  `;

  messagesList.appendChild(el);
  messagesList.scrollTop = messagesList.scrollHeight;
}

// --- Hierarchy links ---

function updateHierarchyLinks() {
  const links = [];
  for (const [, av] of agentViews) {
    if (av._reportsTo) {
      const managerAv = agentViews.get(av._reportsTo);
      if (managerAv) {
        links.push({ from: managerAv, to: av });
      }
    }
  }
  world.hierarchyLinks = links;
}

// --- User question cards ---

const shownQuestions = new Set(); // track displayed question message IDs

function checkForUserQuestions(msgs) {
  for (const msg of msgs) {
    if (msg.type === "user_question" && !shownQuestions.has(msg.id)) {
      shownQuestions.add(msg.id);
      showUserQuestionCard(msg);
    }
  }
}

function showUserQuestionCard(msg) {
  const card = document.createElement("div");
  card.className = "user-question-card";
  card.dataset.msgId = msg.id;

  const fromLabel = msg.from || "agent";
  const subject = msg.subject || "";
  const content = msg.content || "";

  card.innerHTML = `
    <div class="uq-from">${escapeHtml(fromLabel)}</div>
    <div class="uq-subject">${escapeHtml(subject)}</div>
    <div class="uq-content">${escapeHtml(content)}</div>
    <textarea placeholder="Type your response..."></textarea>
    <button>Respond</button>
  `;

  const textarea = card.querySelector("textarea");
  const button = card.querySelector("button");

  button.addEventListener("click", async () => {
    const response = textarea.value.trim();
    if (!response) return;
    button.disabled = true;
    button.textContent = "Sending...";

    const ok = await client.sendUserResponse(client.project, msg.from, response, msg.id);
    if (ok) {
      card.style.opacity = "0";
      card.style.transition = "opacity 0.3s ease";
      setTimeout(() => card.remove(), 300);
    } else {
      button.disabled = false;
      button.textContent = "Respond";
    }
  });

  userQuestionsPanel.appendChild(card);
}

// --- Agent detail panel ---

function openDetail(av) {
  detailPanel.classList.add("open");
  detailName.textContent = av.name;
  detailName.style.color = av.color;
  detailRole.textContent = av.role || "\u2014";
  detailDesc.textContent = av.description || "\u2014";
  detailStatus.textContent = av.online ? "Online" : "Offline";
  detailStatus.style.color = av.online ? "#00e676" : "#636e72";
  detailLastSeen.textContent = formatTime(av._lastSeenRaw);
  detailRegistered.textContent = formatTime(av._registeredRaw);

  // Reports To
  if (av._reportsTo) {
    detailReportsTo.innerHTML = "";
    const link = document.createElement("span");
    link.className = "detail-hierarchy-link";
    link.textContent = av._reportsTo;
    link.addEventListener("click", () => {
      const managerAv = agentViews.get(av._reportsTo);
      if (managerAv) openDetail(managerAv);
    });
    detailReportsTo.appendChild(link);
  } else {
    detailReportsTo.textContent = "\u2014";
  }

  // Direct Reports
  const directReports = [];
  for (const a of agentsData) {
    if (a.reports_to === av.name) {
      directReports.push(a.name);
    }
  }

  if (directReports.length > 0) {
    detailDirectReports.innerHTML = "";
    const container = document.createElement("div");
    container.className = "detail-reports-list";
    for (const name of directReports) {
      const tag = document.createElement("span");
      tag.className = "detail-report-tag";
      tag.textContent = name;
      tag.addEventListener("click", () => {
        const reportAv = agentViews.get(name);
        if (reportAv) openDetail(reportAv);
      });
      container.appendChild(tag);
    }
    detailDirectReports.appendChild(container);
  } else {
    detailDirectReports.textContent = "\u2014";
  }
}

detailClose.addEventListener("click", () => {
  detailPanel.classList.remove("open");
});

// Canvas click → open agent detail
canvas.addEventListener("click", (e) => {
  const rect = canvas.getBoundingClientRect();
  const px = e.clientX - rect.left;
  const py = e.clientY - rect.top;

  for (const [, av] of agentViews) {
    if (av.hitTest(px, py)) {
      openDetail(av);
      return;
    }
  }

  // Click on empty space → close detail
  detailPanel.classList.remove("open");
});

// Hover cursor
canvas.addEventListener("mousemove", (e) => {
  const rect = canvas.getBoundingClientRect();
  const px = e.clientX - rect.left;
  const py = e.clientY - rect.top;

  let hovering = false;
  for (const [, av] of agentViews) {
    if (av.hitTest(px, py)) {
      hovering = true;
      break;
    }
  }
  canvas.style.cursor = hovering ? "pointer" : "default";
});

// Re-layout on resize
window.addEventListener("resize", () => layoutAgents());

// --- Helpers ---

function formatTime(isoStr) {
  if (!isoStr) return "\u2014";
  try {
    const d = new Date(isoStr);
    return d.toLocaleTimeString("en", {
      hour12: false,
      hour: "2-digit",
      minute: "2-digit",
      second: "2-digit",
    });
  } catch {
    return isoStr;
  }
}

function escapeHtml(str) {
  return str
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/"/g, "&quot;");
}

// --- Patch agent metadata ---
// Store raw timestamps on AgentView for the detail panel
const _origOnAgents = onAgents;
function patchedOnAgents(agents) {
  _origOnAgents(agents);
  for (const a of agents) {
    const av = agentViews.get(a.name);
    if (av) {
      av._lastSeenRaw = a.last_seen;
      av._registeredRaw = a.registered_at;
    }
  }
}

// --- Start ---

console.log("[relay] UI initializing...");
const client = new APIClient(patchedOnAgents, onConversations, onNewMessages);

// Load projects first, then start polling
loadProjects().then(() => {
  client.start();
  console.log("[relay] polling started");
  loadConversationMessages();
});
