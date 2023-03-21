<template>
    <div ref="messageBox" id="message-box" class="bg-dark-less fg-white">
        <div v-for="message in messages" class="message w3-padding-small w3-hover-shadow"
            :class="{ topic: message.msg_type === 'TOPIC', privmsg: message.msg_type === 'MUC_PRIVMSG' }">
            <div class="message-meta">
                <FontAwesomeIcon :icon="getMessageIcon(message)" class="w3-text-grey icon"
                    :class="[`icon-${message.msg_type}`]" :title="message.msg_type" />
                <span class="message-time w3-tiny w3-text-grey">
                    {{ formatTime(message.utctime) }}
                </span>
                <b v-if="message.msg_type === 'USER' || message.msg_type === 'MUC_PRIVMSG'"
                    class="message-nick w3-text-green">
                    {{ message.nick }}:
                </b>
                <b v-else-if="message.msg_type === 'TOPIC'" class="message-nick w3-text-green w3-opacity">
                    {{ message.nick }}&ZeroWidthSpace;
                </b>
                <b v-else-if="message.msg_type === 'PART_JOIN'" class="message-nick w3-text-green w3-opacity">
                    {{ message.nick }}
                </b>
                <b v-else-if="message.msg_type === 'PART_LEAVE'" class="message-nick w3-text-green w3-grayscale">
                    {{ message.nick }}
                </b>
            </div>

            <span v-if="message.msg_type === 'USER' || message.msg_type === 'MUC_PRIVMSG'" class="message-text">
                {{ message.text }}
            </span>
            <span v-else-if="message.msg_type === 'TOPIC'" class="message-text w3-text-grey">
                set topic to &OpenCurlyDoubleQuote;{{ message.text }}&CloseCurlyDoubleQuote;
            </span>

            <span v-else-if="message.msg_type === 'PART_JOIN'" class="message-text w3-text-grey">
                joined
            </span>

            <span v-else-if="message.msg_type === 'PART_LEAVE'" class="message-text w3-text-grey">
                leave ({{ message.text.toLowerCase() }})
            </span>
        </div>
    </div>
</template>

<script>
import { library } from '@fortawesome/fontawesome-svg-core'
import { FontAwesomeIcon } from '@fortawesome/vue-fontawesome'
import { faEnvelope, faArrowRightFromBracket, faArrowRightToBracket, faT } from '@fortawesome/free-solid-svg-icons'
import { faCommentDots } from '@fortawesome/free-regular-svg-icons'

import moment from 'moment-timezone'

library.add(faEnvelope, faArrowRightFromBracket, faArrowRightToBracket, faT, faCommentDots);

const getScrollPercent = () => {
    const h = document.documentElement,
        b = document.body,
        st = 'scrollTop',
        sh = 'scrollHeight';
    if ((h[sh] - h.clientHeight - h[st]) < 40) return 100;
    return (h[st] || b[st]) / ((h[sh] || b[sh]) - h.clientHeight) * 100;
}

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
    data() {
        return {
            onPageBottom: false,
        }
    },
    components: { FontAwesomeIcon },
    methods: {
        getMessageIcon(message) {
            switch (message.msg_type) {
                case 'USER': return 'fa-envelope'
                case 'PART_JOIN': return 'fa-arrow-right-to-bracket'
                case 'PART_LEAVE': return 'fa-arrow-right-from-bracket'
                case 'TOPIC': return 'fa-t'
                case 'MUC_PRIVMSG': return 'fa-regular fa-comment-dots'
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
    },
    updated() {
        // Scroll to bottom if we was on page bottom
        if (this.onPageBottom) {
            window.scrollTo({
                top: document.documentElement.scrollHeight,
                behavior: "smooth",
            })
        }
    },
    mounted() {
        window.addEventListener("scroll", () => {
            this.onPageBottom = (getScrollPercent() >= 99.9)
        })

        window.addEventListener("resize", () => {
            this.onPageBottom = (getScrollPercent() >= 99.9)
        })
    },
}
</script>

<style scoped>
#message-box {
    font-family: Arial, Helvetica, sans-serif;
}

.message-time {
    padding-left: .5em;
    padding-right: .5em;
}

.message-meta {
    float: inline-start;
    padding-right: .5em;
}

.message-text {
    display: inline-block;
    white-space: pre-wrap;
}

.topic {
    border: 1px solid #0c0e15;
    background-color: #1f263f;
}

.privmsg {
    background-color: #eeeeee11;
}

.privmsg .message-text {
    color: #aaa;
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

.icon-MUC_PRIVMSG {
    color: skyblue !important;
}
</style>