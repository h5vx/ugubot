<template>
    <div class="w3-container main-header bg-primary-darkest fg-white">
        <button id="menu-button" class="w3-button w3-left w3-large w3-hide-large fg-primary bg-hover-primary-darker"
            @click="$emit('menuClick')">
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
    emits: ["menuClick"],
    components: { FontAwesomeIcon },
}
</script>

<style scoped>
#menu-button {
    margin-left: -15px;
    margin-top: 2px;
}

.main-header {
    border-bottom: 1px solid #0c0e15;
}
</style>