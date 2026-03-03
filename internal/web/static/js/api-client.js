export class APIClient {
  constructor(onAgents, onConversations, onNewMessages) {
    this.onAgents = onAgents;
    this.onConversations = onConversations;
    this.onNewMessages = onNewMessages;

    this.project = "default";
    this._lastMessageTime = null;
    this._agentTimer = null;
    this._msgTimer = null;
    this._convTimer = null;
    this._running = false;
  }

  setProject(p) {
    this.project = p;
    this._lastMessageTime = null; // reset watermark
  }

  start() {
    this._running = true;

    // Initial fetch
    this.fetchAgents();
    this.fetchConversations();

    // Poll agents every 5s
    this._agentTimer = setInterval(() => this.fetchAgents(), 5000);

    // Poll conversations every 10s
    this._convTimer = setInterval(() => this.fetchConversations(), 10000);

    // Poll new messages every 2s
    this._msgTimer = setInterval(() => this.fetchLatestMessages(), 2000);
  }

  stop() {
    this._running = false;
    clearInterval(this._agentTimer);
    clearInterval(this._msgTimer);
    clearInterval(this._convTimer);
  }

  async fetchProjects() {
    try {
      const res = await fetch("/api/projects");
      if (!res.ok) return [];
      return await res.json();
    } catch {
      return [];
    }
  }

  async fetchAgents() {
    try {
      const res = await fetch(`/api/agents?project=${encodeURIComponent(this.project)}`);
      if (!res.ok) return;
      const agents = await res.json();
      this.onAgents(agents);
    } catch (e) {
      console.error("[relay] fetchAgents error:", e);
    }
  }

  async fetchConversations() {
    try {
      const res = await fetch(`/api/conversations?project=${encodeURIComponent(this.project)}`);
      if (!res.ok) return;
      const convs = await res.json();
      this.onConversations(convs);
    } catch (e) {
      console.error("[relay] fetchConversations error:", e);
    }
  }

  async fetchConversationMessages(convId) {
    try {
      const res = await fetch(`/api/conversations/${convId}/messages?project=${encodeURIComponent(this.project)}`);
      if (!res.ok) return [];
      return await res.json();
    } catch {
      return [];
    }
  }

  async fetchAllMessages() {
    try {
      const res = await fetch(`/api/messages/all?project=${encodeURIComponent(this.project)}`);
      if (!res.ok) return [];
      return await res.json();
    } catch {
      return [];
    }
  }

  async fetchLatestMessages() {
    try {
      const since = this._lastMessageTime || new Date(Date.now() - 30000).toISOString();
      const res = await fetch(`/api/messages/latest?since=${encodeURIComponent(since)}&project=${encodeURIComponent(this.project)}`);
      if (!res.ok) return;
      const msgs = await res.json();

      if (msgs.length > 0) {
        // Update watermark to the latest message time
        this._lastMessageTime = msgs[msgs.length - 1].created_at;
        this.onNewMessages(msgs);
      }
    } catch {
      // Silently ignore
    }
  }

  async fetchOrgTree() {
    try {
      const res = await fetch(`/api/org?project=${encodeURIComponent(this.project)}`);
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
}
