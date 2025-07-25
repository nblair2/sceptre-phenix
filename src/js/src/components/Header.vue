<!-- 
The header component is available on all views based on the 
App.vue component. The available routable links are available 
based on whether the user is logged in and furthermore based on 
their role. Based on the current limitations per user role, these 
are only available to Global Administrator or Global Viewer.
 -->

<template>
  <b-navbar class="mb-4">
    <template #brand>
      <b-navbar-item tag="router-link" :to="homeLoc()" :active="false">
        <img src="@/assets/phenix-banner.png" alt="phenix">
      </b-navbar-item>
    </template>
    <template #start>
      <b-navbar-item v-if="auth && roleAllowed('experiments', 'list')" tag="router-link"
        :to="{ name: 'experiments' }">Experiments</b-navbar-item>
      <b-navbar-item v-if="auth && roleAllowed('configs', 'list')" tag="router-link"
        :to="{ name: 'configs' }">Configs</b-navbar-item>
      <b-navbar-item v-if="auth && roleAllowed('disks', 'list')" tag="router-link"
        :to="{name: 'disks'}">Disks</b-navbar-item>
      <b-navbar-item v-if="auth && roleAllowed('hosts', 'list')" tag="router-link"
        :to="{ name: 'hosts' }">Hosts</b-navbar-item>
      <b-navbar-item v-if="auth" tag="router-link"
        :to="{ name: 'users' }">Users</b-navbar-item>
      <b-navbar-item v-if="auth && roleAllowed('logs', 'list')" tag="router-link"
        :to="{ name: 'log' }">Log</b-navbar-item>
      <b-navbar-item v-if="auth && roleAllowed('experiments', 'list')" tag="router-link"
        :to="{ name: 'scorch' }">Scorch</b-navbar-item>
      <b-navbar-item v-if="auth && roleAllowed('experiments', 'list')" tag="router-link"
        :to="builderLoc()" target="_blank">Builder</b-navbar-item>
      <b-navbar-item v-if="auth && roleAllowed('miniconsole', 'post')" tag="router-link"
        :to="{name: 'console'}">Console</b-navbar-item>
      <b-navbar-item v-if="auth && tunneler" tag="router-link"
        :to="{ name: 'tunneler' }">Tunneler</b-navbar-item>
      <b-navbar-item v-if="auth && roleAllowed('settings', 'edit')" tag="router-link"
        :to="{name: 'settings'}">Settings</b-navbar-item>  
    </template>

    <template #end>
      <b-navbar-item v-if="proxyAuth" class="navbar-item" @click="logout">Reauthorize
      </b-navbar-item>
      <b-navbar-item v-else-if="auth" class="navbar-item" @click="logout">Logout
      </b-navbar-item>
    </template>
  </b-navbar>

</template>

<script>
import MenuLink from '@/components/MenuLink.vue'

export default {
  components: {
    menuLink: MenuLink
  },

  //  The computed elements determine if the user is already logged 
  //  in; if so, the routable links are available. If not, the sign 
  //  in routable link is the only one available. The role getter 
  //  determines what the role of the user is; this is used to present 
  //  routable links in the header row.
  computed: {
    auth() {
      return this.$store.getters.auth;
    },

    proxyAuth() {
      return process.env.VUE_APP_AUTH === 'proxy';
    },

    tunneler() {
      return this.$store.getters.features.includes('tunneler-download');
    }
  },

  methods: {
    //  These methods are used to logout a user; or, present 
    //  routable link based on a Global user role.
    logout() {
      this.$http.get('logout').then(
        response => {
          if (response.status == 204) {
            this.$store.commit('LOGOUT');

            if (this.proxyAuth) {
              this.$buefy.toast.open({
                message: 'Your account has been reauthorized',
                type: 'is-success',
                duration: 4000
              });
            }
          }
        }
      );
    },

    homeLoc() {
      return this.$router.resolve({ name: 'home' }).href
    },

    builderLoc() {
      return this.$router.resolve({ name: 'builder', params: { token: this.$store.getters.token } }).href;
    }
  }
}
</script>
