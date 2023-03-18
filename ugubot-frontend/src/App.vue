<template>
  <TheChatListSidebar :chats="chats" :active-chat-id="activeChatId" @chatSelected="onChatSelected" />

  <main id="main" style="margin-left: 286px; transition: none;">
    <TheHeader :text="activeChat.jid" />
    <TheDatePicker :dates="chatDates[activeChatId]" :current-date="getCurrentDate()" />
  </main>
</template>

<script>
import TheChatListSidebar from './components/TheChatListSidebar.vue'
import TheHeader from './components/TheHeader.vue'
import TheDatePicker from './components/TheDatePicker.vue';
import moment from 'moment'

export default {
  components: { TheChatListSidebar, TheHeader, TheDatePicker },
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
        }
      }
    }
  },
  methods: {
    onChatSelected(chatId) {
      this.activeChatId = chatId
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

<style scoped></style>
