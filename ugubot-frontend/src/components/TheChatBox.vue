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
                <b :class="getClassesForNick(message)" @click="this.$emit('nickClick', { e: $event, nick: message.nick })">
                    {{ message.nick }}{{ message.msg_type === 'USER' ? ':' : '' }}
                </b>
            </div>

            <span v-if="message.msg_type === 'USER' || message.msg_type === 'MUC_PRIVMSG'" class="message-text"
                v-html="linkify(message.text)">
            </span>
            <span v-else-if="message.msg_type === 'TOPIC'" class="message-text w3-text-grey"
                v-html="'set topic to «' + linkify(message.text) + '»'">
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
import { faCommentDots } from '@fortawesome/free-regular-svg-icons'
import { faArrowRightFromBracket, faArrowRightToBracket, faEnvelope, faL, faT } from '@fortawesome/free-solid-svg-icons'
import { FontAwesomeIcon } from '@fortawesome/vue-fontawesome'
import { escapeHtml } from '@vue/shared'

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
    emits: ["nickClick"],
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
        getClassesForNick(message) {
            let result = ["message-nick", `message-nick-${message.nick}`]
            switch (message.msg_type) {
                case 'PART_JOIN':
                case 'TOPIC':
                    result.push("w3-opacity")
                    break
                case 'PART_LEAVE':
                    result.push("w3-grayscale")
                    break
            }
            return result
        },
        formatTime(timestamp) {
            // Working with timezones using momentjs is absolute bullshit
            // I spent half an hour trying to do UTC timestamp conversion to local time
            // and at the end just fuck it, so we adding utcoffset manually to timestamp
            const utcOffset = moment.tz(this.tz).utcOffset()
            const localDate = moment(timestamp + utcOffset * 60 * 1000)
            return localDate.format('HH:mm:ss')
        },
        linkify(s) {
            const urlRegex = /(https?:\/\/[^\s]+)/g;
            const content = escapeHtml(s)
            return content.replace(urlRegex, '<a href="$1" class="message-link" target="_blank">$1</a>')
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

<style>
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

.message-link {
    text-decoration: none;
    color: #5371d5;
}

.message-link:hover {
    text-decoration: underline;
}

.message-link:visited {
    color: #8c97bc;
}

.message-nick {
    cursor: pointer;
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