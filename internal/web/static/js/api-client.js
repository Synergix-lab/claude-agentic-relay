export class APIClient {
  constructor(onAgents, onConversations, onNewMessages, onNewTasks, onActivity) {
    this.onAgents = onAgents;
    this.onConversations = onConversations;
    this.onNewMessages = onNewMessages;
    this.onNewTasks = onNewTasks;
    this.onActivity = onActivity;

    this._lastMessageTime = null;
    this._lastTaskTime = null;
    this._agentTimer = null;
    this._msgTimer = null;
    this._convTimer = null;
    this._taskTimer = null;
    this._goalTimer = null;
    this._running = false;
    this.onGoals = null;
  }

  start() {
    this._running = true;

    // Initial fetch (cross-project)
    this.fetchAllAgents();
    this.fetchAllConversations();
    this.fetchAllTasks().then(tasks => {
      if (this.onNewTasks && tasks.length > 0) this.onNewTasks(tasks);
    });

    // Poll agents every 5s (structural changes only, SSE handles status)
    this._agentTimer = setInterval(() => this.fetchAllAgents(), 5000);

    // Poll conversations every 10s
    this._convTimer = setInterval(() => this.fetchAllConversations(), 10000);

    // Poll new messages every 2s
    this._msgTimer = setInterval(() => this.fetchLatestMessagesAllProjects(), 2000);

    // Poll tasks every 3s
    this._taskTimer = setInterval(() => this.fetchLatestTasks(), 3000);

    // Poll goals every 10s
    this._goalTimer = setInterval(() => this._refreshGoals(), 10000);

    // SSE for real-time activity + agent status (<100ms)
    this._sseConnected = false;
    this._activitySource = new EventSource("/api/activity/stream");
    this._activitySource.onopen = () => {
      console.log("[relay] SSE connected");
      this._sseConnected = true;
      // Kill fallback polling if SSE reconnects
      if (this._activityTimer) {
        clearInterval(this._activityTimer);
        this._activityTimer = null;
      }
    };
    this._activitySource.onmessage = (e) => {
      try {
        const payload = JSON.parse(e.data);
        if (payload.sessions && payload.agents) {
          if (this.onActivity) this.onActivity(payload.sessions, payload.agents);
        } else {
          if (this.onActivity) this.onActivity(payload, null);
        }
      } catch (err) {
        console.error("[relay] SSE parse error:", err);
      }
    };
    this._activitySource.onerror = (e) => {
      console.warn("[relay] SSE error, state:", this._activitySource.readyState);
      // Only fallback if SSE is fully closed (readyState === 2)
      if (this._activitySource.readyState === 2 && !this._activityTimer) {
        console.log("[relay] SSE closed, falling back to polling");
        this._activityTimer = setInterval(() => this.fetchActivity(), 1000);
      }
    };
  }

  stop() {
    this._running = false;
    clearInterval(this._agentTimer);
    clearInterval(this._msgTimer);
    clearInterval(this._convTimer);
    clearInterval(this._taskTimer);
    clearInterval(this._goalTimer);
    if (this._activitySource) this._activitySource.close();
    clearInterval(this._activityTimer);
  }

  async fetchAllAgents() {
    try {
      const res = await fetch("/api/agents/all");
      if (!res.ok) return;
      const agents = await res.json();
      this.onAgents(agents);
    } catch (e) {
      console.error("[relay] fetchAllAgents error:", e);
    }
  }

  async fetchAllConversations() {
    try {
      const res = await fetch("/api/conversations/all");
      if (!res.ok) return;
      const convs = await res.json();
      this.onConversations(convs);
    } catch (e) {
      console.error("[relay] fetchAllConversations error:", e);
    }
  }

  async fetchLatestMessagesAllProjects() {
    try {
      const since = this._lastMessageTime || new Date(Date.now() - 30000).toISOString();
      const res = await fetch(`/api/messages/latest-all?since=${encodeURIComponent(since)}`);
      if (!res.ok) return;
      const msgs = await res.json();

      if (msgs.length > 0) {
        this._lastMessageTime = msgs[msgs.length - 1].created_at;
        this.onNewMessages(msgs);
      }
    } catch {
      // Silently ignore
    }
  }

  async fetchAllMessagesAllProjects() {
    try {
      const res = await fetch("/api/messages/all-projects");
      if (!res.ok) return [];
      return await res.json();
    } catch {
      return [];
    }
  }

  async fetchConversationMessages(convId) {
    try {
      const res = await fetch(`/api/conversations/${convId}/messages`);
      if (!res.ok) return [];
      return await res.json();
    } catch {
      return [];
    }
  }

  async sendUserResponse(project, to, content, replyTo) {
    try {
      const res = await fetch("/api/user-response", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ project, to, content, reply_to: replyTo }),
      });
      return res.ok;
    } catch {
      return false;
    }
  }

  async fetchActivity() {
    try {
      const res = await fetch("/api/activity");
      if (!res.ok) return;
      const sessions = await res.json();
      if (this.onActivity) this.onActivity(sessions);
    } catch {
      // Silently ignore
    }
  }

  // --- Memory API ---

  async fetchMemories(params = {}) {
    try {
      const qs = new URLSearchParams();
      if (params.project) qs.set("project", params.project);
      if (params.scope) qs.set("scope", params.scope);
      if (params.agent) qs.set("agent", params.agent);
      if (params.tag) qs.set("tag", params.tag);
      const res = await fetch(`/api/memories?${qs}`);
      if (!res.ok) return [];
      return await res.json();
    } catch {
      return [];
    }
  }

  async searchMemories(query) {
    try {
      const res = await fetch(`/api/memories/search?q=${encodeURIComponent(query)}`);
      if (!res.ok) return [];
      return await res.json();
    } catch {
      return [];
    }
  }

  async createMemory(data) {
    try {
      const res = await fetch("/api/memories", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(data),
      });
      return res.ok ? await res.json() : null;
    } catch {
      return null;
    }
  }

  async deleteMemory(id) {
    try {
      const res = await fetch(`/api/memories/${id}`, { method: "DELETE" });
      return res.ok;
    } catch {
      return false;
    }
  }

  async resolveConflict(key, chosenValue, project, scope) {
    try {
      const res = await fetch(`/api/memories/${encodeURIComponent(key)}/resolve`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ chosen_value: chosenValue, project, scope }),
      });
      return res.ok ? await res.json() : null;
    } catch {
      return null;
    }
  }

  async _refreshGoals() {
    if (this.onGoals) {
      const goals = await this.fetchAllGoals();
      this.onGoals(goals);
    }
  }

  // --- Goal API ---

  async fetchAllGoals() {
    try {
      const res = await fetch("/api/goals/all");
      if (!res.ok) return [];
      return await res.json();
    } catch {
      return [];
    }
  }

  async fetchGoals(params = {}) {
    try {
      const qs = new URLSearchParams();
      if (params.project) qs.set("project", params.project);
      if (params.type) qs.set("type", params.type);
      if (params.status) qs.set("status", params.status);
      const res = await fetch(`/api/goals?${qs}`);
      if (!res.ok) return [];
      return await res.json();
    } catch {
      return [];
    }
  }

  async fetchGoalCascade(project) {
    try {
      const qs = project ? `?project=${encodeURIComponent(project)}` : "";
      const res = await fetch(`/api/goals/cascade${qs}`);
      if (!res.ok) return [];
      return await res.json();
    } catch {
      return [];
    }
  }

  async createGoal(data) {
    try {
      const res = await fetch("/api/goals", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(data),
      });
      if (!res.ok) return null;
      return await res.json();
    } catch {
      return null;
    }
  }

  async updateGoal(goalId, data) {
    try {
      const res = await fetch(`/api/goals/${goalId}`, {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(data),
      });
      if (!res.ok) return null;
      return await res.json();
    } catch {
      return null;
    }
  }

  // --- Task API ---

  async fetchAllTasks() {
    try {
      const res = await fetch("/api/tasks/all");
      if (!res.ok) return [];
      return await res.json();
    } catch {
      return [];
    }
  }

  async fetchTasks(params = {}) {
    try {
      const qs = new URLSearchParams();
      if (params.project) qs.set("project", params.project);
      if (params.status) qs.set("status", params.status);
      if (params.profile) qs.set("profile", params.profile);
      if (params.priority) qs.set("priority", params.priority);
      const res = await fetch(`/api/tasks?${qs}`);
      if (!res.ok) return [];
      return await res.json();
    } catch {
      return [];
    }
  }

  async fetchLatestTasks() {
    try {
      const since = this._lastTaskTime || new Date(Date.now() - 30000).toISOString();
      const res = await fetch(`/api/tasks/latest?since=${encodeURIComponent(since)}`);
      if (!res.ok) return;
      const tasks = await res.json();
      if (tasks.length > 0) {
        this._lastTaskTime = tasks[tasks.length - 1].dispatched_at;
        if (this.onNewTasks) this.onNewTasks(tasks);
      }
    } catch {
      // Silently ignore
    }
  }

  async dispatchTask(data) {
    try {
      const res = await fetch("/api/tasks", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(data),
      });
      if (!res.ok) return null;
      return await res.json();
    } catch {
      return null;
    }
  }

  async transitionTask(taskId, status, project, agent, result, reason) {
    try {
      const body = { status, project: project || "default", agent: agent || "user" };
      if (result) body.result = result;
      if (reason) body.reason = reason;
      const res = await fetch(`/api/tasks/${taskId}/transition`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(body),
      });
      if (!res.ok) return null;
      return await res.json();
    } catch {
      return null;
    }
  }

  async cancelTask(taskId, project, agent) {
    return this.transitionTask(taskId, "cancelled", project, agent);
  }

  async fetchAllTeams() {
    try {
      const res = await fetch("/api/teams/all");
      if (!res.ok) return [];
      return await res.json();
    } catch {
      return [];
    }
  }

  async fetchBoards(project) {
    try {
      const res = await fetch(`/api/boards?project=${encodeURIComponent(project)}`);
      if (!res.ok) return [];
      return await res.json();
    } catch {
      return [];
    }
  }

  async fetchAllBoards() {
    try {
      const res = await fetch("/api/boards/all");
      if (!res.ok) return [];
      return await res.json();
    } catch {
      return [];
    }
  }

  async updateTask(taskId, data) {
    try {
      const res = await fetch(`/api/tasks/${taskId}`, {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(data),
      });
      if (!res.ok) return null;
      return await res.json();
    } catch {
      return null;
    }
  }

  async deleteTask(taskId, project) {
    try {
      const qs = project ? `?project=${encodeURIComponent(project)}` : "";
      const res = await fetch(`/api/tasks/${taskId}${qs}`, { method: "DELETE" });
      return res.ok;
    } catch {
      return false;
    }
  }

  async fetchTask(taskId, project) {
    try {
      const qs = project ? `?project=${encodeURIComponent(project)}` : "";
      const res = await fetch(`/api/tasks/${taskId}${qs}`);
      if (!res.ok) return null;
      return await res.json();
    } catch {
      return null;
    }
  }

  // --- Vault API ---

  async fetchAllVaultDocs() {
    try {
      const res = await fetch("/api/vault/docs/all");
      if (!res.ok) return [];
      return await res.json();
    } catch {
      return [];
    }
  }

  async fetchVaultDocs(project, tags) {
    try {
      const qs = new URLSearchParams();
      if (project) qs.set("project", project);
      if (tags) qs.set("tags", JSON.stringify(tags));
      const res = await fetch(`/api/vault/docs?${qs}`);
      if (!res.ok) return [];
      return await res.json();
    } catch {
      return [];
    }
  }

  async searchVaultDocs(project, query) {
    try {
      const qs = new URLSearchParams();
      if (project) qs.set("project", project);
      qs.set("q", query);
      const res = await fetch(`/api/vault/search?${qs}`);
      if (!res.ok) return { results: [] };
      return await res.json();
    } catch {
      return { results: [] };
    }
  }

  async fetchVaultDoc(project, path) {
    try {
      const qs = project ? `?project=${encodeURIComponent(project)}` : "";
      const encodedPath = path.split("/").map(encodeURIComponent).join("/");
      const res = await fetch(`/api/vault/doc/${encodedPath}${qs}`);
      if (!res.ok) return null;
      return await res.json();
    } catch {
      return null;
    }
  }

  async updateVaultDoc(project, path, content) {
    try {
      const qs = project ? `?project=${encodeURIComponent(project)}` : "";
      const encodedPath = path.split("/").map(encodeURIComponent).join("/");
      const res = await fetch(`/api/vault/doc/${encodedPath}${qs}`, {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ content }),
      });
      return res.ok;
    } catch {
      return false;
    }
  }

  // --- Triggers API ---

  async fetchTriggers(project) {
    try {
      const qs = project ? `?project=${encodeURIComponent(project)}` : '';
      const res = await fetch(`/api/triggers${qs}`);
      if (!res.ok) return [];
      return await res.json();
    } catch { return []; }
  }

  async fetchTriggerHistory(project) {
    try {
      const qs = project ? `?project=${encodeURIComponent(project)}` : '';
      const res = await fetch(`/api/trigger-history${qs}`);
      if (!res.ok) return [];
      return await res.json();
    } catch { return []; }
  }

  async createTrigger(data) {
    try {
      const res = await fetch('/api/triggers', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(data),
      });
      return res.ok ? await res.json() : null;
    } catch { return null; }
  }

  async deleteTrigger(id) {
    try {
      const res = await fetch(`/api/triggers/${id}`, { method: 'DELETE' });
      return res.ok;
    } catch { return false; }
  }

  // --- Poll Triggers API ---

  async fetchPollTriggers(project) {
    try {
      const qs = project ? `?project=${encodeURIComponent(project)}` : '';
      const res = await fetch(`/api/poll-triggers${qs}`);
      if (!res.ok) return [];
      return await res.json();
    } catch { return []; }
  }

  async createPollTrigger(data) {
    try {
      const res = await fetch('/api/poll-triggers', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(data),
      });
      return res.ok ? await res.json() : null;
    } catch { return null; }
  }

  async deletePollTrigger(id) {
    try {
      const res = await fetch(`/api/poll-triggers/${id}`, { method: 'DELETE' });
      return res.ok;
    } catch { return false; }
  }

  async testPollTrigger(id) {
    try {
      const res = await fetch(`/api/poll-triggers/${id}/test`, { method: 'POST' });
      if (!res.ok) return null;
      return await res.json();
    } catch { return null; }
  }

  // --- Skills API ---

  async fetchSkills(project, agent) {
    try {
      let qs = project ? `?project=${encodeURIComponent(project)}` : '';
      if (agent) qs += `&agent=${encodeURIComponent(agent)}`;
      const res = await fetch(`/api/skills${qs}`);
      if (!res.ok) return [];
      return await res.json();
    } catch { return []; }
  }

  async createSkill(data) {
    try {
      const res = await fetch('/api/skills', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(data),
      });
      return res.ok ? await res.json() : null;
    } catch { return null; }
  }

  async fetchSkillProfiles(name, project) {
    try {
      const qs = project ? `?project=${encodeURIComponent(project)}` : '';
      const res = await fetch(`/api/skills/${encodeURIComponent(name)}/profiles${qs}`);
      if (!res.ok) return [];
      return await res.json();
    } catch { return []; }
  }

  // --- Quotas API ---

  async fetchQuotas(project) {
    try {
      const qs = project ? `?project=${encodeURIComponent(project)}` : '';
      const res = await fetch(`/api/quotas${qs}`);
      if (!res.ok) return [];
      return await res.json();
    } catch { return []; }
  }

  async fetchAgentQuota(agent, project) {
    try {
      const qs = project ? `?project=${encodeURIComponent(project)}` : '';
      const res = await fetch(`/api/quotas/${encodeURIComponent(agent)}${qs}`);
      if (!res.ok) return null;
      return await res.json();
    } catch { return null; }
  }

  async updateAgentQuota(agent, data) {
    try {
      const res = await fetch(`/api/quotas/${encodeURIComponent(agent)}`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(data),
      });
      return res.ok ? await res.json() : null;
    } catch { return null; }
  }

  // --- Profiles API ---

  async fetchProfiles(project) {
    try {
      const qs = project ? `?project=${encodeURIComponent(project)}` : '';
      const res = await fetch(`/api/profiles${qs}`);
      if (!res.ok) return [];
      return await res.json();
    } catch { return []; }
  }

  async fetchProfile(slug, project) {
    try {
      const qs = project ? `?project=${encodeURIComponent(project)}` : '';
      const res = await fetch(`/api/profiles/${encodeURIComponent(slug)}${qs}`);
      if (!res.ok) return null;
      return await res.json();
    } catch { return null; }
  }

  async createProfile(data) {
    try {
      const res = await fetch('/api/profiles', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(data),
      });
      return res.ok ? await res.json() : null;
    } catch { return null; }
  }

  async updateProfile(slug, data) {
    try {
      const qs = data.project ? `?project=${encodeURIComponent(data.project)}` : '';
      const res = await fetch(`/api/profiles/${encodeURIComponent(slug)}${qs}`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(data),
      });
      return res.ok ? await res.json() : null;
    } catch { return null; }
  }

  async deleteProfile(slug, project) {
    try {
      const qs = project ? `?project=${encodeURIComponent(project)}` : '';
      const res = await fetch(`/api/profiles/${encodeURIComponent(slug)}${qs}`, { method: 'DELETE' });
      return res.ok;
    } catch { return false; }
  }

  async deactivateAgent(name, project) {
    try {
      const qs = project ? `?project=${encodeURIComponent(project)}` : '';
      const res = await fetch(`/api/agents/${encodeURIComponent(name)}${qs}`, { method: 'DELETE' });
      return res.ok;
    } catch { return false; }
  }

  async deleteSkill(name, project) {
    try {
      const qs = project ? `?project=${encodeURIComponent(project)}` : '';
      const res = await fetch(`/api/skills/${encodeURIComponent(name)}${qs}`, { method: 'DELETE' });
      return res.ok;
    } catch { return false; }
  }

  async deleteQuota(agent, project) {
    try {
      const qs = project ? `?project=${encodeURIComponent(project)}` : '';
      const res = await fetch(`/api/quotas/${encodeURIComponent(agent)}${qs}`, { method: 'DELETE' });
      return res.ok;
    } catch { return false; }
  }

  // --- Service Discovery API ---

  async discoverBySkill(project, skill) {
    try {
      const qs = new URLSearchParams();
      if (project) qs.set('project', project);
      if (skill) qs.set('skill', skill);
      const res = await fetch(`/api/discover?${qs}`);
      if (!res.ok) return null;
      return await res.json();
    } catch { return null; }
  }

  // --- Elevations API ---

  async fetchElevations(project) {
    try {
      const qs = project ? `?project=${encodeURIComponent(project)}` : '';
      const res = await fetch(`/api/elevations${qs}`);
      if (!res.ok) return [];
      return await res.json();
    } catch { return []; }
  }

  async grantElevation(data) {
    try {
      const res = await fetch('/api/elevations', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(data),
      });
      return res.ok ? await res.json() : null;
    } catch { return null; }
  }

  async revokeElevation(id) {
    try {
      const res = await fetch(`/api/elevations/${id}`, { method: 'DELETE' });
      return res.ok;
    } catch { return false; }
  }

  // --- Projects API ---

  async fetchProjects() {
    try {
      const res = await fetch("/api/projects");
      if (!res.ok) return [];
      return await res.json();
    } catch {
      return [];
    }
  }

  async fetchProject(name) {
    try {
      const res = await fetch(`/api/projects/${encodeURIComponent(name)}`);
      if (!res.ok) return null;
      return await res.json();
    } catch {
      return null;
    }
  }

  async updateProjectPlanet(name, planetType) {
    try {
      const res = await fetch(`/api/projects/${encodeURIComponent(name)}`, {
        method: "PATCH",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ planet_type: planetType }),
      });
      return res.ok;
    } catch { return false; }
  }

  async fetchSettings() {
    try {
      const res = await fetch("/api/settings");
      if (!res.ok) return {};
      return await res.json();
    } catch { return {}; }
  }

  async updateSettings(settings) {
    try {
      const res = await fetch("/api/settings", {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(settings),
      });
      return res.ok;
    } catch { return false; }
  }

  async fetchFileLocks(project) {
    try {
      const qs = project ? `?project=${encodeURIComponent(project)}` : "";
      const res = await fetch(`/api/file-locks${qs}`);
      if (!res.ok) return [];
      return await res.json();
    } catch {
      return [];
    }
  }

  async fetchVaultStats(project) {
    try {
      const qs = project ? `?project=${encodeURIComponent(project)}` : "";
      const res = await fetch(`/api/vault/stats${qs}`);
      if (!res.ok) return null;
      return await res.json();
    } catch {
      return null;
    }
  }

  async fetchTokenUsage(period = "24h") {
    try {
      const res = await fetch(`/api/token-usage?period=${encodeURIComponent(period)}`);
      if (!res.ok) return [];
      return await res.json();
    } catch { return []; }
  }

  async fetchTokenUsageByProject(project, period = "24h") {
    try {
      const res = await fetch(`/api/token-usage/project?project=${encodeURIComponent(project)}&period=${encodeURIComponent(period)}`);
      if (!res.ok) return [];
      return await res.json();
    } catch { return []; }
  }

  async fetchTokenUsageByAgent(project, agent, period = "24h") {
    try {
      const qs = `project=${encodeURIComponent(project)}&period=${encodeURIComponent(period)}`;
      const agentQs = agent ? `&agent=${encodeURIComponent(agent)}` : "";
      const res = await fetch(`/api/token-usage/agent?${qs}${agentQs}`);
      if (!res.ok) return [];
      return await res.json();
    } catch { return []; }
  }

  async fetchTokenTimeSeries(project, period = "24h", agent = "") {
    try {
      let qs = `project=${encodeURIComponent(project)}&period=${encodeURIComponent(period)}`;
      if (agent) qs += `&agent=${encodeURIComponent(agent)}`;
      const res = await fetch(`/api/token-usage/timeseries?${qs}`);
      if (!res.ok) return [];
      return await res.json();
    } catch { return []; }
  }

  // --- Cycles API ---

  async fetchCycles(project) {
    try {
      const qs = project ? `?project=${encodeURIComponent(project)}` : '';
      const res = await fetch(`/api/cycles${qs}`);
      if (!res.ok) return [];
      return await res.json();
    } catch { return []; }
  }

  async createCycle(data) {
    try {
      const res = await fetch('/api/cycles', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(data),
      });
      return res.ok ? await res.json() : null;
    } catch { return null; }
  }

  async updateCycle(name, data) {
    try {
      const res = await fetch(`/api/cycles/${encodeURIComponent(name)}`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(data),
      });
      return res.ok ? await res.json() : null;
    } catch { return null; }
  }

  async deleteCycle(name, project) {
    try {
      const qs = project ? `?project=${encodeURIComponent(project)}` : '';
      const res = await fetch(`/api/cycles/${encodeURIComponent(name)}${qs}`, { method: 'DELETE' });
      return res.ok;
    } catch { return false; }
  }

  // --- Workflows API ---

  async fetchWorkflows(project) {
    try {
      const qs = project ? `?project=${encodeURIComponent(project)}` : '';
      const res = await fetch(`/api/workflows${qs}`);
      if (!res.ok) return [];
      return await res.json();
    } catch { return []; }
  }

  async fetchWorkflow(id) {
    try {
      const res = await fetch(`/api/workflows/${encodeURIComponent(id)}`);
      if (!res.ok) return null;
      return await res.json();
    } catch { return null; }
  }

  async createWorkflow(data) {
    try {
      const res = await fetch('/api/workflows', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(data),
      });
      return res.ok ? await res.json() : null;
    } catch { return null; }
  }

  async updateWorkflow(id, data) {
    try {
      const res = await fetch(`/api/workflows/${encodeURIComponent(id)}`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(data),
      });
      return res.ok ? await res.json() : null;
    } catch { return null; }
  }

  async deleteWorkflow(id) {
    try {
      const res = await fetch(`/api/workflows/${encodeURIComponent(id)}`, { method: 'DELETE' });
      return res.ok;
    } catch { return false; }
  }

  async executeWorkflow(id, meta = {}) {
    try {
      const res = await fetch(`/api/workflows/${encodeURIComponent(id)}/execute`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ meta }),
      });
      return res.ok ? await res.json() : null;
    } catch { return null; }
  }

  async fetchWorkflowRuns(workflowId, limit = 20) {
    try {
      const res = await fetch(`/api/workflows/${encodeURIComponent(workflowId)}/runs?limit=${limit}`);
      if (!res.ok) return [];
      return await res.json();
    } catch { return []; }
  }

  async fetchWorkflowRunDetail(runId) {
    try {
      const res = await fetch(`/api/workflow-runs/${encodeURIComponent(runId)}`);
      if (!res.ok) return null;
      return await res.json();
    } catch { return null; }
  }

  // --- Schedules API ---

  async fetchSchedules(project) {
    try {
      const qs = project ? `?project=${encodeURIComponent(project)}` : '';
      const res = await fetch(`/api/schedules${qs}`);
      if (!res.ok) return [];
      return await res.json();
    } catch { return []; }
  }

  async fetchAgentSchedules(project, agent) {
    try {
      const qs = new URLSearchParams();
      if (project) qs.set('project', project);
      if (agent) qs.set('agent', agent);
      const res = await fetch(`/api/schedules?${qs}`);
      if (!res.ok) return [];
      return await res.json();
    } catch { return []; }
  }

  async createSchedule(data) {
    try {
      const res = await fetch('/api/schedules', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(data),
      });
      return res.ok ? await res.json() : null;
    } catch { return null; }
  }

  async updateSchedule(id, data) {
    try {
      const res = await fetch(`/api/schedules/${encodeURIComponent(id)}`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(data),
      });
      return res.ok ? await res.json() : null;
    } catch { return null; }
  }

  async deleteSchedule(id) {
    try {
      const res = await fetch(`/api/schedules/${encodeURIComponent(id)}`, { method: 'DELETE' });
      return res.ok;
    } catch { return false; }
  }

  async triggerSchedule(id) {
    try {
      const res = await fetch(`/api/schedules/${encodeURIComponent(id)}/trigger`, { method: 'POST' });
      return res.ok ? await res.json() : null;
    } catch { return null; }
  }

  // --- Spawn API ---

  async fetchCycleHistory(project, agent) {
    try {
      const qs = new URLSearchParams();
      if (project) qs.set('project', project);
      if (agent) qs.set('agent', agent);
      const res = await fetch(`/api/cycle-history?${qs}`);
      if (!res.ok) return [];
      return await res.json();
    } catch { return []; }
  }

  async fetchSpawnChildren(project, agent, status) {
    try {
      const qs = new URLSearchParams();
      if (project) qs.set('project', project);
      if (agent) qs.set('agent', agent);
      if (status) qs.set('status', status);
      const res = await fetch(`/api/spawn/children?${qs}`);
      if (!res.ok) return [];
      return await res.json();
    } catch { return []; }
  }

  async killSpawnChild(id) {
    try {
      const res = await fetch(`/api/spawn/children/${encodeURIComponent(id)}/kill`, { method: 'POST' });
      return res.ok;
    } catch { return false; }
  }

  async spawnWithContext(data) {
    try {
      const res = await fetch('/api/spawn/context', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(data),
      });
      return res.ok ? await res.json() : null;
    } catch { return null; }
  }

  async terminalSpawn(data) {
    try {
      const res = await fetch('/api/terminal/spawn', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(data),
      });
      return res.ok ? await res.json() : null;
    } catch { return null; }
  }

  async terminalKill(sessionId) {
    try {
      const res = await fetch(`/api/terminal/${encodeURIComponent(sessionId)}/kill`, { method: 'POST' });
      return res.ok;
    } catch { return false; }
  }

  terminalWsUrl(sessionId) {
    const proto = location.protocol === 'https:' ? 'wss:' : 'ws:';
    return `${proto}//${location.host}/api/terminal/ws/${sessionId}`;
  }
}
