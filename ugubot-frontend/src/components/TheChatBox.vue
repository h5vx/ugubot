<template>
    <div id="message-box" class="bg-dark-less fg-white">
        <div v-for="message in messages" class="message w3-padding-small w3-hover-shadow"
            :class="{ topic: message.msg_type === 'TOPIC' }">
            <FontAwesomeIcon :icon="getMessageIcon(message)" class="w3-text-grey icon" :class="[`icon-${message.msg_type}`]" />
            <span class="message-time w3-tiny w3-text-grey">
                {{ formatTime(message.utctime) }}
            </span>
            <b v-if="message.msg_type === 'USER'" class="message-nick w3-text-green">
                {{ message.nick }}:
            </b>
            <span v-if="message.msg_type === 'USER'" class="message-text">{{ message.text }}</span>
            <span v-else-if="message.msg_type === 'TOPIC'" class="message-text">{{ message.text }}</span>
            <span v-else-if="message.msg_type === 'PART_JOIN'" class="message-text w3-text-grey">
                <b class="w3-text-green w3-opacity">{{ message.nick }}</b> joined
            </span>
            <span v-else-if="message.msg_type === 'PART_LEAVE'" class="message-text w3-text-grey">
                <b class="w3-text-green w3-grayscale">{{ message.nick }}</b> leave
            </span>
        </div>
    </div>
</template>

<script>
import { library } from '@fortawesome/fontawesome-svg-core'
import { FontAwesomeIcon } from '@fortawesome/vue-fontawesome'
import { faEnvelope, faArrowRightFromBracket, faArrowRightToBracket, faT } from '@fortawesome/free-solid-svg-icons'
import moment from 'moment-timezone'

library.add(faEnvelope, faArrowRightFromBracket, faArrowRightToBracket, faT);

export default {
    props: {
        messages: {
            type: Array,
            required: true
        },
        tz: {
            type: String,
            required: true
        },
    },
    components: { FontAwesomeIcon },
    methods: {
        getMessageIcon(message) {
            switch (message.msg_type) {
                case 'USER': return 'fa-envelope'
                case 'PART_JOIN': return 'fa-arrow-right-to-bracket'
                case 'PART_LEAVE': return 'fa-arrow-right-from-bracket'
                case 'TOPIC': return 'fa-t'
            }
        },
        formatTime(timestamp) {
            // Working with timezones using momentjs is absolute bullshit
            // I spent half an hour trying to do UTC timestamp conversion to local time
            // and at the end just fuck it, so we adding utcoffset manually to timestamp
            const utcOffset = moment.tz(this.tz).utcOffset()
            const localDate = moment(timestamp + utcOffset * 60 * 1000)
            return localDate.format('HH:mm:ss')
        },
    }
}
</script>

<style scoped>
#message-box {
    margin-bottom: 42px;
    font-family: Arial, Helvetica, sans-serif;
}

.message-time {
    padding-left: .5em;
    padding-right: .5em;
}

.topic {
    border: 1px solid #0c0e15;
    background-color: #1f263f;
}

.icon {
    min-width: 1em;
}

.icon-USER {
    color: #7089dd !important;
}

.icon-PART_JOIN {
    color: greenyellow !important;
}

.icon-PART_LEAVE {
    color: red !important;
}

</style>