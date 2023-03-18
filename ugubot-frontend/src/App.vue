<template>
  <TheChatListSidebar ref="sidebar" :chats="chats" :active-chat-id="activeChatId" @chatSelected="onChatSelected"
    @menuClick="onMenuClick" />

  <main id="main" style="margin-left: 286px; transition: none;">
    <TheHeader :text="activeChat.jid" />
    <TheDatePicker :dates="chatDates[activeChatId]" :current-date="getCurrentDate()" @date-selected="onDateSelected" />
    <TheChatBox :messages="chatMessages" />
  </main>
</template>

<script>
import TheChatListSidebar from './components/TheChatListSidebar.vue'
import TheHeader from './components/TheHeader.vue'
import TheDatePicker from './components/TheDatePicker.vue'
import TheChatBox from './components/TheChatBox.vue'
import moment from 'moment'

export default {
  components: { TheChatListSidebar, TheHeader, TheDatePicker, TheChatBox },
  data() {
    return {
      activeChatId: 0,
      chats: [
        { "id": 0, "type": "muc", "jid": "boardplay@conference.0-chan.ru", "name": "boardplay@conference.jabber.ru" },
        { "id": 1, "type": "user", "jid": "ugubok@ugubok.ru", "name": "ugubok" },
        { "id": 2, "type": "user", "jid": "lorem@ugubok.ru", "name": "lorem" },
      ],
      chatDates: {
        0: {
          "2021": {
            "Jan": ["01", "02", "03", "04", "05", "06", "07", "08"]
          },
          "2022": {
            "Jan": ["01", "02", "03", "04", "05", "06", "07", "08"]
          },
          "2023": {
            "Jan": ["01", "02", "03", "04", "05", "06", "07", "08", "09", "10"],
            "Feb": ["01", "02", "03", "04", "05", "06", "07", "08", "09", "10", "11", "12", "13", "14", "15", "16", "17", "18", "19", "20", "21", "22", "23", "24", "25"],
            "Mar": ["01", "02", "03", "04", "05", "06", "07", "08", "09", "10", "11", "12", "13", "14", "15", "16", "17", "18"],
          },
        },
        1: { "2021": { "Jan": ["01"] } },
        2: { "2021": { "Jan": ["01"] } },
      },
      chatMessages: [
        { type: "message", date: moment(1360013296123), nick: "mya", text: "Hello" },
        { type: "message", date: moment(1360013296123), nick: "alice", text: "Hi" },
        { type: "join", date: moment(1360013296123), nick: "Greek", text: "Greek joined" },
        { type: "message", date: moment(1360013296123), nick: "Greek", text: "lorem" },
        { type: "leave", date: moment(1360013296123), nick: "Greek", text: "Greek leave" },
        { type: "message", date: moment(1360013296123), nick: "mya", text: "Hello" },
        { type: "message", date: moment(1360013296123), nick: "alice", text: "Hi" },
        { type: "join", date: moment(1360013296123), nick: "Greek", text: "Greek joined" },
        { type: "message", date: moment(1360013296123), nick: "Greek", text: "lorem" },
        { type: "leave", date: moment(1360013296123), nick: "Greek", text: "Greek leave" },
        { type: "message", date: moment(1360013296123), nick: "mya", text: "Hello" },
        { type: "message", date: moment(1360013296123), nick: "alice", text: "Hi" },
        { type: "join", date: moment(1360013296123), nick: "Greek", text: "Greek joined" },
        { type: "message", date: moment(1360013296123), nick: "Greek", text: "lorem" },
        { type: "leave", date: moment(1360013296123), nick: "Greek", text: "Greek leave" },
        { type: "message", date: moment(1360013296123), nick: "mya", text: "Hello" },
        { type: "message", date: moment(1360013296123), nick: "alice", text: "Hi" },
        { type: "join", date: moment(1360013296123), nick: "Greek", text: "Greek joined" },
        { type: "message", date: moment(1360013296123), nick: "Greek", text: "lorem" },
        { type: "leave", date: moment(1360013296123), nick: "Greek", text: "Greek leave" },
        { type: "message", date: moment(1360013296123), nick: "mya", text: "Hello" },
        { type: "message", date: moment(1360013296123), nick: "alice", text: "Hi" },
        { type: "join", date: moment(1360013296123), nick: "Greek", text: "Greek joined" },
        { type: "message", date: moment(1360013296123), nick: "Greek", text: "lorem" },
        { type: "leave", date: moment(1360013296123), nick: "Greek", text: "Greek leave" },
        { type: "message", date: moment(1360013296123), nick: "mya", text: "Hello" },
        { type: "message", date: moment(1360013296123), nick: "alice", text: "Hi" },
        { type: "join", date: moment(1360013296123), nick: "Greek", text: "Greek joined" },
        { type: "message", date: moment(1360013296123), nick: "Greek", text: "lorem" },
        { type: "leave", date: moment(1360013296123), nick: "Greek", text: "Greek leave" },
      ],
    }
  },
  methods: {
    onChatSelected(chatId) {
      this.activeChatId = chatId
    },
    onDateSelected(date) {
      console.log(date.format("YYYY/MMM/DD"))
    },
    onMenuClick() {
      this.$refs.sidebar.style.display = "block"
    },
    getCurrentDate() {
      return moment()
    },
  },
  computed: {
    activeChat() {
      return this.chats[this.activeChatId]
    }
  },
  mounted() {
  },
};
</script>

<style scoped>

@media (max-width:993px) {
  #main {
    margin-left: 0px !important;
  }
}
</style>
