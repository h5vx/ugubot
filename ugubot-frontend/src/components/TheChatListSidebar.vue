<template>
    <aside id="sidebar" class="w3-sidebar w3-collapse bg-dark fg-white">
        <h2 class="w3-center w3-wide w3-large bg-dark fg-cherry">
            CHATLIST
        </h2>

        <button id="sidebar-close-button" class="w3-button w3-display-topright w3-hide-large w3-hover-none">
            <FontAwesomeIcon icon="fa-close"></FontAwesomeIcon>
        </button>

        <div class="chatlist">
            <h6 v-for="chat in chats" :class="{
                selected: chat.id === activeChatId,
                'w3-hover-opacity-off': chat.id !== activeChatId,
                'w3-opacity': chat.id !== activeChatId,
            }" @click="$emit('chatSelected', chat.id)">
                <FontAwesomeIcon :icon="chat.type === 'muc' ? 'fa-comments' : 'fa-user'" :class="{
                    chaticon: true,
                    'w3-text-red': chat.id === activeChatId,
                    'w3-text-grey': chat.id !== activeChatId,
                }" />
                {{ chat.name }}
            </h6>
        </div>

        <div class="resizer-handle" min-width="144" max-width="400">

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
        }
    },
    components: { FontAwesomeIcon },
    emits: ["chatSelected"],
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
</style>