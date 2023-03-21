<template>
    <div ref="container" class="w3-bottom w3-card-2">
        <textarea v-model="input" ref="input" rows="1" type="text" class="w3-input fg-white" name="message"
            placeholder="Write a message…" id="message-input" @input="onInput"
            @keydown.enter.shift.exact.prevent="sendMessage"></textarea>
        <button v-if="input.length > 0" class="w3-button w3-right w3-hover-none fg-primary-light fg-hover-white"
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

let mainResizeObserver, messageBoxResizeObserver

export default {
    components: { FontAwesomeIcon },
    emits: ["message"],
    data() {
        return {
            input: ""
        }
    },
    mounted() {
        const main = document.getElementById("main")
        const resizeInput = () => {
            if (!this.$refs.input) return
            this.$refs.input.style.width = (main.clientWidth - 100) + "px"
            this.$refs.container.style.width = main.clientWidth + "px"
        }

        const resizeMessageBox = () => {
            const mb = document.getElementById("message-box")
            if (!mb) return

            let e = document.scrollingElement
            mb.style.marginBottom = this.$refs.container.clientHeight + "px"

            if ((e.scrollHeight - e.clientHeight - e.scrollTop) < 40) {
                window.scrollTo({
                    top: e.scrollHeight,
                    behavior: "auto",
                })
            }
        }

        resizeInput()
        resizeMessageBox()

        mainResizeObserver = new ResizeObserver(resizeInput).observe(main)
        messageBoxResizeObserver = new ResizeObserver(resizeMessageBox).observe(this.$refs.container)
    },
    beforeUnmount() {
        const mb = document.getElementById("message-box")
        if (!mb) return

        mb.style.marginBottom = null

        if (mainResizeObserver) mainResizeObserver.unobserve()
        if (messageBoxResizeObserver) messageBoxResizeObserver.unobserve()
    },
    methods: {
        onInput(e) {
            e.target.style.height = "auto"
            e.target.style.height = e.target.scrollHeight + "px"
        },
        sendMessage() {
            this.$emit("message", this.input)
            this.input = ""
            this.$refs.input.style.height = "auto"
        }
    },
}
</script>

<style scoped>
textarea {
    max-height: 84px;
    outline: none;
    resize: none;
    background-color: #222941;
    float: left;
    padding-right: 0px;
    border-bottom: none;
}

div {
    background-color: #222941;
    border-top: 1px solid #2f3a5e;
}

#send-icon {
    font-size: 1.5em;
    margin-bottom: -4px;
}
</style>