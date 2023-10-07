<template>
    <div ref="container" class="textbox w3-card-2">
        <textarea v-model="input" ref="input" rows="1" type="text" spellcheck="false" class="w3-input fg-white"
            name="message" placeholder="Write a message…" id="message-input" @input="onInput"
            @keydown.enter.shift.exact.prevent="addNewLine" @keydown.enter.exact.prevent="sendMessage"></textarea>
        <button v-if="input.length > 0" class="send-button w3-button w3-right w3-hover-none fg-primary-light fg-hover-white"
            @click="sendMessage">
            <FontAwesomeIcon id="send-icon" icon="fa-solid fa-angles-right"></FontAwesomeIcon>
        </button>
    </div>
</template>

<script>
import { library } from '@fortawesome/fontawesome-svg-core'
import { FontAwesomeIcon } from '@fortawesome/vue-fontawesome'
import { faAnglesRight } from '@fortawesome/free-solid-svg-icons'

library.add(faAnglesRight)

export default {
    components: { FontAwesomeIcon },
    emits: ["message"],
    data() {
        return {
            input: ""
        }
    },
    mounted() {
    },
    beforeUnmount() {
    },
    methods: {
        onInput(e) {
            e.target.style.height = "auto"
            e.target.style.height = e.target.scrollHeight + "px"
        },
        addNewLine(e) {
            e.target.value += "\n"
            this.onInput(e)
        },
        sendMessage() {
            this.$emit("message", this.input)
            this.input = ""
            this.$refs.input.style.height = "auto"
        },
    },
}
</script>

<style scoped>
.textbox {
    position: sticky;
    display: flex;
    bottom: 0;
}
textarea {
    flex: 1 1 auto;
    max-height: 84px;
    outline: none;
    resize: none;
    background-color: #222941;
    padding-right: 0px;
    border-bottom: none;
}

div {
    background-color: #222941;
    border-top: 1px solid #2f3a5e;
}
.send-button {
    flex: 0 1 auto;
    align-self: flex-start;
}
#send-icon {
    font-size: 1.5em;
    margin-bottom: -4px;
}
</style>
