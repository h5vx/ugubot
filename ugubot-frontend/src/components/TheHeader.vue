<template>
    <div class="w3-container main-header bg-primary-darkest fg-white">
        <button id="menu-button" class="w3-button w3-left w3-large w3-hide-large fg-primary">
            <FontAwesomeIcon icon="fa-bars"></FontAwesomeIcon>
        </button>
        <h4 class="w3-large">{{ text }}</h4>
    </div>
</template>

<script>
import { library } from '@fortawesome/fontawesome-svg-core'
import { FontAwesomeIcon } from '@fortawesome/vue-fontawesome'
import { faBars } from '@fortawesome/free-solid-svg-icons'

library.add(faBars)

export default {
    props: {
        text: {
            type: String,
            required: true,
        },
    },
    components: { FontAwesomeIcon },
}
</script>