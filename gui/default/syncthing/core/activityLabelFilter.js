angular.module('syncthing.core')
    .filter('activityLabel', function () {
        var labels = {
            'file_create': 'File Created',
            'file_modify': 'File Modified',
            'file_delete': 'File Deleted',
            'file_share': 'File Shared',
            'sync_complete': 'Sync Complete',
            'user_login': 'Login'
        };
        return function (input) {
            return labels[input] || input;
        };
    });
