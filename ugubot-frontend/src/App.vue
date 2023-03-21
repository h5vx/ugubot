<template>
  <!-- <div v-if="!connected" class="w3-container w3-center">
    <h1 class="fg-primary w3-animate-fading">Connecting...</h1>

    <div id="logs" class="w3-container bg-dark fg-white w3-card-2">
      <p v-for="log in startupScreenLogs" class="log-entry" :style="{ color: log.color }">
        {{ log.text }}
      </p>
    </div>
  </div> -->

  <TheChatListSidebar ref="sidebar" :chats="chats" :active-chat-id="activeChatId" @chatSelected="onChatSelected" />

  <main id="main" style="margin-left: 286px; transition: none;">
    <TheHeader :text="activeChat.jid" @menuClick="onMenuClick" />
    <TheDatePicker :dates="activeChatDates" :current-date="getCurrentDate()" @date-selected="onDateSelected" />
    <TheChatBox :messages="chatMessages" :tz="tz" />
    <TheInputPrompt />
  </main>
</template>

<script>
import TheChatListSidebar from './components/TheChatListSidebar.vue'
import TheHeader from './components/TheHeader.vue'
import TheDatePicker from './components/TheDatePicker.vue'
import TheChatBox from './components/TheChatBox.vue'
import TheInputPrompt from './components/TheInputPrompt.vue'
import moment from 'moment-timezone'

const chatPlaceholder = { id: 0, type: "muc", jid: "...", name: "..." }

export default {
  components: { TheChatListSidebar, TheHeader, TheDatePicker, TheChatBox, TheInputPrompt },
  data() {
    return {
      ws: null,
      tz: moment.tz.guess(),
      connected: false,
      activeChatId: 0,
      startupScreenLogs: [],
      chats: [
        // { "id": 0, "type": "muc", "jid": "some chat", "name": "some chat" },
      ],
      chatDates: {
        // 1: { "...": { "...": ["..."] } },
      },
      chatMessages: [
        // { type: "message", date: moment(1360013296123), nick: "mya", text: "Hello" },
        // { type: "join", date: moment(1360013296123), nick: "Greek", text: "Greek joined" },
        // { type: "leave", date: moment(1360013296123), nick: "Greek", text: "Greek leave" },
      ],
    }
  },
  methods: {
    onChatSelected(chatId) {
      // Request chat dates if selected chat dates doesn't present
      if (!(chatId in this.chatDates)) {
        this.ws.send(JSON.stringify({
          command: "get_dates",
          client_timezone: this.tz
        }))
      }
      this.activeChatId = chatId
    },
    onDateSelected(date) {
      this.ws.send(JSON.stringify({
        command: "get_messages",
        date: date.format("YYYY/MM/DD"),
        chat_id: this.activeChatId,
        client_timezone: this.tz
      }))
    },
    onMenuClick() {
      this.$refs.sidebar.$el.style.display = "block"
    },
    onWebSocketConnected(e) {
      this.addLog("Connected", "green")
      this.addLog("Receiving chats")
      this.updateChats()
      this.connected = true
    },
    onWebSocketDisconnected(e) {
      this.addLog("Connection lost", "red")
      this.connected = false
    },
    onWebSocketMessage(e) {
      let data = JSON.parse(e.data)

      switch (data.command) {
        case "get_chat_list":
          this.chats = data.result
          break
        case "get_dates":
          this.chatDates = data.result
          break
        case "get_messages":
          if (data.chat_id !== this.activeChatId) {
            console.warn(`Rejected messages for chat ${data.chat_id} because active chat is ${this.activeChatId}`)
            return
          }
          this.chatMessages = data.result
          break
        case "new_message":
          this.handleNewMessage(data.message)
          break
        default:
          console.log("Unknown command:", data)
      }
    },
    updateChats() {
      this.ws.send(JSON.stringify({
        command: "get_chat_list"
      }))
    },
    handleNewMessage(message) {
      const utcOffset = moment.tz(this.tz).utcOffset()
      const localDate = moment(message.utctime + utcOffset * 60 * 1000)
      const year = localDate.format("YYYY")
      const month = localDate.format("MMM")
      const day = localDate.format("DD")

      const chatExists = this.chats.filter(c => c.id === message.chat).length() > 0

      if (!chatExists) {
        this.chats[message.chat] = chatPlaceholder
        this.updateChats()
      }
    },
    getCurrentDate() {
      return moment()
    },
    addLog(text, color = '#eee') {
      this.startupScreenLogs.push({ text: text, color: color })
    },
    getCookie(name) {
      const value = `; ${document.cookie}`;
      const parts = value.split(`; ${name}=`);
      if (parts.length === 2) return parts.pop().split(';').shift();
    },
    connectWebSocket() {
      this.addLog("Acknowledge websocket url...")
      fetch('/ws_url').then(response => {
        let wsUrl = response.json().url || "ws://localhost:8000/ws"
        this.addLog(`Connecting to ${wsUrl}`)
        this.ws = new WebSocket(wsUrl + "?token=" + this.getCookie("session"))
        this.ws.onopen = this.onWebSocketConnected
        this.ws.onmessage = this.onWebSocketMessage
        this.ws.onclose = this.onWebSocketDisconnected
      })
    }
  },
  computed: {
    activeChat() {
      if (this.activeChatId !== null) {
        let result = this.chats.filter(c => c.id === this.activeChatId)
        if (result.length !== 0) {
          return result[0]
        }
      }
      return chatPlaceholder
    },
    activeChatDates() {
      if (this.activeChatId in this.chatDates) {
        return this.chatDates[this.activeChatId]
      }
      return { "...": { "...": ["..."] } }
    },
  },
  mounted() {
    this.connectWebSocket()
  },
};
</script>

<style scoped>
@media (max-width:993px) {
  #main {
    margin-left: 0px !important;
  }
}

#logs {
  width: 80%;
  margin: auto;
  max-width: 800px;
  min-height: 200px;
  padding-top: 8px;
}

.log-entry {
  display: block;
  margin: 0;
  text-align: left;
}
</style>
