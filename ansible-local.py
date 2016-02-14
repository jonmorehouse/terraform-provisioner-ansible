#!/usr/bin/env python
#vim:syntax=python

import argparse
import collections
import json
import os.path
import socket
import subprocess
import sys

from ansible.vars import VariableManager
from ansible.parsing.dataloader import DataLoader
from ansible.inventory import Inventory
from ansible.inventory import Host
from ansible.inventory import Group
from ansible.playbook import Playbook
from ansible.executor.task_queue_manager import TaskQueueManager


OptionsClass = collections.namedtuple('Options', ('connection', 'module_path', 'forks', 'remote_user', 'private_key_file',
                                 'ssh_common_args', 'ssh_extra_args', 'sftp_extra_args', 'scp_extra_args',
                                 'become', 'become_method', 'become_user', 'verbosity', 'check'))


def build_inventory(loader, variable_manager, group_names):
    inventory = Inventory(loader=loader, variable_manager=variable_manager, host_list=['localhost'])
    host = inventory.get_hosts()[0]
    
    for group_name in group_names:
        if not inventory.get_group(group_name):
            group = Group(group_name)
            inventory.add_group(group)

        inventory.get_group(group_name).add_host(host)

    return inventory


def build_plays(loader, variable_manager, playbook_path, play_ids):
    playbook = Playbook.load(playbook_path, variable_manager, loader)
    plays = []
    
    for play in playbook.get_plays():
        # a play can correspond to a play name or the host group
        for play_id in play_ids:
            if play.get_name() == play_id:
                plays.append(play)
            elif play._ds['hosts'] == play_id:
                plays.append(play)

    return plays


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description='ansible-local script for running ansible against localhost')

    parser.add_argument('--module-path', help='path to ansible source', required=True)
    parser.add_argument('--playbook', help='path to playbook file', required=True)
    parser.add_argument('--plays', help='list of plays to apply to host', required=True, type=lambda x: x.split(","))
    parser.add_argument('--groups', help='list of groups to apply to host', required=True, type=lambda x: x.split(","))
    parser.add_argument('--extra-vars', help='json encoded string with extra-var information', required=True, type=lambda x: json.loads(x))

    args = parser.parse_args()
    
    options = OptionsClass(connection='local',
                      module_path=args.module_path,
                      forks=100,
                      remote_user=None,
                      private_key_file=None,
                      ssh_common_args=None,
                      ssh_extra_args=None,
                      sftp_extra_args=None,
                      scp_extra_args=None,
                      become=None,
                      become_method='sudo',
                      become_user='root',
                      verbosity=None,
                      check=False)

    loader = DataLoader()
    variable_manager = VariableManager()
    variable_manager.extra_vars = {
        'hosts': 'all',
        'roles': 'all',
    }
    variable_manager.extra_vars.update(args.extra_vars)
    
    inventory = build_inventory(loader, variable_manager, args.groups + args.plays)

    variable_manager.set_inventory(inventory)
    plays = build_plays(loader, variable_manager, args.playbook, args.plays)

    tqm = TaskQueueManager(
        inventory=inventory,
        variable_manager=variable_manager,
        loader=loader,
        passwords=None,
        options=options,
        stdout_callback='default',
        run_tree=False,
    )

    for play in plays:
        tqm.run(play)
