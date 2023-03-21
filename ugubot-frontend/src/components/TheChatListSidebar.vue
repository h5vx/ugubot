<template>
    <aside ref="sidebar" id="sidebar" class="w3-sidebar w3-collapse bg-dark fg-white">
        <h2 class="w3-center w3-wide w3-large bg-dark fg-cherry">
            CHATLIST
        </h2>

        <button id="sidebar-close-button" class="w3-button w3-display-topright w3-hide-large w3-hover-none"
            @click="hideSidebar">
            <FontAwesomeIcon icon="fa-close"></FontAwesomeIcon>
        </button>

        <div class="chatlist">
            <h6 v-for="chat in chats" :class="{
                selected: chat.id === activeChatId,
                'w3-hover-opacity-off': chat.id !== activeChatId,
                'w3-opacity': chat.id !== activeChatId,
            }" @click="$emit('chatSelected', chat.id)">
                <div class="chaticon">
                    <FontAwesomeIcon :icon="chat.type === 'muc' ? 'fa-comments' : 'fa-user'" :class="{
                        'w3-text-red': chat.id === activeChatId,
                        'w3-text-grey': chat.id !== activeChatId,
                    }" />
                    <span v-if="unreadIds.has(chat.id)" class="unread-badge"></span>
                </div>
                {{ chat.name }}
            </h6>
        </div>

        <div ref="resizeHandle" class="resizer-handle" min-width="144" max-width="400">

        </div>
    </aside>
</template>

<script>
import { library } from '@fortawesome/fontawesome-svg-core'
import { FontAwesomeIcon } from '@fortawesome/vue-fontawesome'
import { faClose, faComments, faUser } from '@fortawesome/free-solid-svg-icons'

library.add(faClose, faComments, faUser)

export default {
    props: {
        chats: {
            type: Array,
            required: true,
        },
        activeChatId: {
            type: Number,
            required: true,
        },
        unreadIds: {
            type: Set,
            required: true,
        },
    },
    components: { FontAwesomeIcon },
    emits: ["chatSelected"],
    methods: {
        setupResizeHandle(el) {
            const resizable = el.parentElement
            const minWidth = Number(el.attributes["min-width"].value)
            const maxWidth = Number(el.attributes["max-width"].value)
            const main = document.getElementById("main")
            const body = document.getElementsByTagName("body")[0]

            const preventTextSelection = () => {
                body.style.userSelect = "none"
            }

            const allowTextSelection = () => {
                body.style.userSelect = null
            }

            const onResize = (e) => {
                if (e.pageX > maxWidth || e.pageX < minWidth) {
                    return
                }

                resizable.style.width = e.pageX + "px"
                main.style.marginLeft = e.pageX + "px"
            }

            const stopResize = (e) => {
                window.removeEventListener("mousemove", onResize)
                allowTextSelection()
                localStorage.sidebarWidth = resizable.style.width
            }

            const startResize = (e) => {
                window.addEventListener("mousemove", onResize)
                window.addEventListener("mouseup", stopResize)
                preventTextSelection()
            }

            el.addEventListener("mousedown", startResize)

            if ("sidebarWidth" in localStorage) {
                resizable.style.width = localStorage.sidebarWidth
                main.style.marginLeft = localStorage.sidebarWidth
            }
        },
        hideSidebar() {
            this.$refs.sidebar.style.display = "none";
        },
    },
    mounted() {
        this.setupResizeHandle(this.$refs.resizeHandle)
    },
}
</script>

<style scoped>
aside {
    z-index: 10;
    width: 286px;
}

#sidebar-close-button {
    padding: 12px 18px 12px 18px;
}

.chatlist h6 {
    padding: 4px 0px 4px 7px;
    margin: 0;
    cursor: pointer;
    white-space: nowrap;
    font-size: 15px;
    font-family: 'Cantarell', sans-serif !important;
    border-top: 1px solid transparent;
    border-bottom: 1px solid transparent;
}

.chatlist .selected {
    background-color: #4d052d;
    border-top: 1px solid #f5118f;
    border-bottom: 1px solid #f5118f;
    color: white;
    font-family: 'Cantarell', sans-serif !important;
}

.chaticon {
    display: inline-block;
    position: relative;
    min-width: 24px;
    padding-right: 5px;
    text-align: center;
}

.resizer-handle {
    position: absolute;
    height: 100%;
    width: 2px;
    top: 0;
    right: 0;
    background-color: #374161;
    float: right;
    cursor: e-resize;
}

#sidebar-close-button {
    padding: 12px 18px 12px 18px;
}

.unread-badge {
    background-color: red;
    border: 1px solid #0c0e15;
    border-radius: 50%;
    font-size: 10px;
    position: absolute;
    bottom: 0;
    right: 2px;
    padding: 4px 4px;
}
</style>