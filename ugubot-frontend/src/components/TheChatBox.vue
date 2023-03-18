<template>
    <div id="message-box" class="bg-dark-less fg-white">
        <div v-for="message in messages" class="message w3-padding-small w3-hover-shadow">
            <FontAwesomeIcon :icon="getMessageIcon(message)" class="w3-text-grey" />
            <span class="message-time w3-tiny w3-text-grey">
                {{ message.date.format("HH:mm:ss") }}
            </span>
            <b v-if="message.type === 'message'" class="message-nick w3-text-green">
                {{ message.nick }}:
            </b>
            <span v-if="message.type === 'message'" class="message-text">{{ message.text }}</span>
            <span v-else-if="message.type === 'join'" class="message-text w3-text-grey">
                <b class="w3-text-brown">{{ message.nick }}</b> joined
            </span>
            <span v-else-if="message.type === 'leave'" class="message-text w3-text-grey">
                <b class="w3-text-brown w3-grayscale">{{ message.nick }}</b> leave
            </span>
        </div>
    </div>
</template>

<script>
import { library } from '@fortawesome/fontawesome-svg-core'
import { FontAwesomeIcon } from '@fortawesome/vue-fontawesome'
import { faEnvelope, faArrowRightFromBracket, faArrowRightToBracket } from '@fortawesome/free-solid-svg-icons'

library.add(faEnvelope, faArrowRightFromBracket, faArrowRightToBracket);

export default {
    props: {
        messages: {
            type: Array,
            required: true
        }
    },
    components: { FontAwesomeIcon },
    methods: {
        getMessageIcon(message) {
            switch (message.type) {
                case 'message': return 'fa-envelope'
                case 'join': return 'fa-arrow-right-to-bracket'
                case 'leave': return 'fa-arrow-right-from-bracket'
            }
        }
    }
}
</script>

<style scoped>
#message-box {
    margin-bottom: 43px;
    font-family: Arial, Helvetica, sans-serif;
}

.message-time {
    padding-left: .5em;
    padding-right: .5em;
}
</style>